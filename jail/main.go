// x39jail: roda um programa do cartão SD numa "jaula" sem acesso à NAND do X39.
//
// Como funciona (processo pai, root, fora da jaula):
//  1. monta um tmpfs (RAM) em /tmp/x39jail e outro em /tmp/x39jail/dev;
//  2. cria em /dev só os nós necessários (nunca nand*, mmcblk* nem mem);
//  3. aplica os binds pedidos (-ro/-rw) e copia este binário para dentro;
//  4. executa a si mesmo com chroot na jaula ("child").
//
// O filho (ainda root, mas já dentro da jaula) remove todas as capabilities,
// vira o usuário 1000, liga no_new_privs e só então roda o teste ou o programa.
// Ao final o pai desmonta tudo. Nada é gravado na memória interna.
//
// Uso:
//
//	x39jail test [-dev]
//	x39jail clean
//	x39jail run [-t segundos] [-env K=V] [-ro origem=destino] [-rw origem=destino] [-dev] [-input /dev/input/jsN] -- /caminho/na/jaula args...
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	jail = "/tmp/x39jail"
	uid  = 1000
	gid  = 1000

	prCapbsetRead   = 23
	prCapbsetDrop   = 24
	prSetNoNewPrivs = 38
	prGetNoNewPrivs = 39
	maxCap          = 63
)

type node struct {
	name         string
	mode         uint32
	major, minor int
	owned        bool // pertence ao usuário da jaula (0660)
}

var baseNodes = []node{
	{"null", syscall.S_IFCHR | 0666, 1, 3, false},
	{"zero", syscall.S_IFCHR | 0666, 1, 5, false},
	{"urandom", syscall.S_IFCHR | 0666, 1, 9, false},
}

// Nós do X39 (números lidos do próprio console). Só vídeo, entrada e som.
var deviceNodes = []node{
	{"tty", syscall.S_IFCHR | 0666, 5, 0, false},
	{"tty0", syscall.S_IFCHR | 0660, 4, 0, true},
	{"console", syscall.S_IFCHR | 0660, 5, 1, true},
	{"fb0", syscall.S_IFCHR | 0660, 29, 0, true},
	{"input/event0", syscall.S_IFCHR | 0660, 13, 64, true},
	{"input/event1", syscall.S_IFCHR | 0660, 13, 65, true},
	{"input/js0", syscall.S_IFCHR | 0660, 13, 0, true},
	{"snd/controlC0", syscall.S_IFCHR | 0660, 116, 0, true},
	{"snd/controlC1", syscall.S_IFCHR | 0660, 116, 32, true},
	{"snd/pcmC0D0p", syscall.S_IFCHR | 0660, 116, 16, true},
	{"snd/pcmC0D0c", syscall.S_IFCHR | 0660, 116, 24, true},
	{"snd/pcmC1D0p", syscall.S_IFCHR | 0660, 116, 48, true},
	{"snd/timer", syscall.S_IFCHR | 0660, 116, 33, true},
}

// extraEnv são as variáveis -env K=V passadas ao programa da jaula.
var extraEnv []string

// extraInputs são nós de entrada extras pedidos com -input (controles USB): origem no host e nome na jaula.
var extraInputs []struct{ src, name string }

var inputPathRe = regexp.MustCompile(`^/dev/input/(js|event)[0-9]{1,3}$`)
var inputNameRe = regexp.MustCompile(`^(js|event)[0-9]{1,3}$`)

// validInputPath diz se p é um caminho aceitável para -input (só js/event de /dev/input, nada de /dev/nand*, mem etc.).
func validInputPath(p string) bool { return inputPathRe.MatchString(p) }

// parseInputArg separa "origem" ou "origem=nome": o nome (js1, event2...) é como o nó aparece DENTRO da jaula.
func parseInputArg(a string) (src, name string, ok bool) {
	src = a
	if i := strings.Index(a, "="); i >= 0 {
		src, name = a[:i], a[i+1:]
		if !inputNameRe.MatchString(name) || !strings.HasPrefix(src, "/dev/input/"+name[:strings.IndexAny(name, "0123456789")]) {
			return "", "", false
		}
	}
	if !validInputPath(src) {
		return "", "", false
	}
	if name == "" {
		name = src[len("/dev/input/"):]
	}
	return src, name, true
}

const inputMajor = 13 // grupo "input" do kernel (event*, js*, mouse*)

// runTimeout, se > 0, encerra o programa da jaula depois desse tempo (-t N).
var runTimeout time.Duration

type bind struct {
	src, dst string
	rw       bool
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "x39jail: "+format+"\n", a...)
	os.Exit(2)
}

func main() {
	if len(os.Args) < 2 {
		die("uso: x39jail test | run [-ro a=b] [-rw a=b] [-dev] -- cmd...")
	}
	switch os.Args[1] {
	case "test":
		// "test -dev" usa os mesmos nós de dispositivo dos jogos (vídeo, entrada, som, terminal)
		os.Exit(parent(len(os.Args) > 2 && os.Args[2] == "-dev", nil, []string{"child-test"}))
	case "run":
		devs, binds, cmd := parseRun(os.Args[2:])
		os.Exit(parent(devs, binds, append([]string{"child-run"}, cmd...)))
	case "clean":
		if os.Geteuid() != 0 {
			die("precisa rodar como root")
		}
		n := cleanMounts()
		fmt.Printf("desmontados: %d\n", n)
	case "features":
		// o lançador pergunta o que esta jaula sabe fazer (uma jaula antiga não tem este comando)
		fmt.Println("input")
	case "child-test":
		childPrepare()
		os.Exit(selfTest())
	case "child-run":
		childPrepare()
		runTarget(os.Args[2:])
	default:
		die("comando desconhecido: %s", os.Args[1])
	}
}

func parseRun(args []string) (devs bool, binds []bind, cmd []string) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-dev":
			devs = true
		case "-input":
			var src, name string
			var ok bool
			if i+1 < len(args) {
				src, name, ok = parseInputArg(args[i+1])
			}
			if !ok || len(extraInputs) >= 6 {
				die("-input precisa de /dev/input/jsN[=jsM] ou /dev/input/eventN[=eventM] (no máximo 6)")
			}
			extraInputs = append(extraInputs, struct{ src, name string }{src, name})
			devs = true // dispositivos extras só fazem sentido com os nós de dispositivo
			i++
		case "-env":
			if i+1 >= len(args) || !strings.Contains(args[i+1], "=") {
				die("-env precisa de CHAVE=VALOR")
			}
			extraEnv = append(extraEnv, args[i+1])
			i++
		case "-t":
			if i+1 >= len(args) {
				die("-t precisa de segundos")
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil || n <= 0 || n > 3600 {
				die("-t inválido: %q", args[i+1])
			}
			runTimeout = time.Duration(n) * time.Second
			i++
		case "-ro", "-rw":
			if i+1 >= len(args) {
				die("%s precisa de origem=destino", args[i])
			}
			p := strings.SplitN(args[i+1], "=", 2)
			if len(p) != 2 || !strings.HasPrefix(p[1], "/") {
				die("bind inválido: %q (use origem=/destino)", args[i+1])
			}
			binds = append(binds, bind{p[0], p[1], args[i] == "-rw"})
			i++
		case "--":
			cmd = args[i+1:]
			i = len(args)
		default:
			die("argumento desconhecido: %s", args[i])
		}
	}
	if len(cmd) == 0 {
		die("faltou o comando depois de --")
	}
	return
}

// ---------------------------------------------------------------- pai

func parent(devs bool, binds []bind, childArgs []string) int {
	if os.Geteuid() != 0 {
		die("precisa rodar como root para montar a jaula")
	}
	// Pdeathsig vale para a thread que criou o filho: mantém esta thread fixa.
	runtime.LockOSThread()

	self, err := os.Executable()
	if err != nil {
		die("executable: %v", err)
	}
	cleanup := func() { cleanMounts() }
	defer cleanup()

	must := func(err error, what string) {
		if err != nil {
			cleanup()
			die("%s: %v", what, err)
		}
	}
	mount := func(src, dst, fstype string, flags uintptr, data string) {
		must(syscall.Mount(src, dst, fstype, flags, data), "mount "+dst)
	}

	if _, err := os.Stat(jail); err == nil {
		die("%s já existe (execução anterior interrompida?); rode: x39jail clean", jail)
	}
	must(os.Mkdir(jail, 0755), "mkdir jaula")
	mount("tmpfs", jail, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "size=8m,mode=0755")

	must(copyFile(self, filepath.Join(jail, "x39jail")), "copiar binário")

	devDir := filepath.Join(jail, "dev")
	must(os.Mkdir(devDir, 0755), "mkdir dev")
	mount("tmpfs", devDir, "tmpfs", syscall.MS_NOSUID|syscall.MS_NOEXEC, "size=64k,mode=0755")
	nodes := baseNodes
	if devs {
		nodes = append(append([]node{}, baseNodes...), deviceNodes...)
	}
	for _, n := range nodes {
		p := filepath.Join(devDir, n.name)
		must(os.MkdirAll(filepath.Dir(p), 0755), "mkdir "+filepath.Dir(p))
		must(syscall.Mknod(p, n.mode, n.major<<8|n.minor), "mknod "+n.name)
		if n.owned {
			must(syscall.Chown(p, uid, gid), "chown "+n.name)
		}
	}

	// controles USB (-input): recria SÓ esse nó, depois de conferir no host que é mesmo um dispositivo de entrada
	for _, in := range extraInputs {
		var st syscall.Stat_t
		must(syscall.Stat(in.src, &st), "stat "+in.src)
		if st.Mode&syscall.S_IFMT != syscall.S_IFCHR || int((st.Rdev>>8)&0xfff) != inputMajor {
			cleanup()
			die("%s não é um dispositivo de entrada (char, grupo %d)", in.src, inputMajor)
		}
		dst := filepath.Join(devDir, "input", in.name)
		if _, err := os.Stat(dst); err == nil {
			continue // já existe (js0, event0, event1)
		}
		must(os.MkdirAll(filepath.Dir(dst), 0755), "mkdir input")
		must(syscall.Mknod(dst, syscall.S_IFCHR|0660, int(st.Rdev)), "mknod "+in.src)
		must(syscall.Chown(dst, uid, gid), "chown "+in.src)
	}

	// /tmp em RAM para o programa (logs, cache); 1777 como num /tmp normal.
	tmpDir := filepath.Join(jail, "tmp")
	must(os.Mkdir(tmpDir, 0755), "mkdir tmp")
	mount("tmpfs", tmpDir, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "size=8m,mode=1777")

	for _, b := range binds {
		dst := filepath.Join(jail, b.dst)
		must(os.MkdirAll(dst, 0755), "mkdir "+dst)
		mount(b.src, dst, "", syscall.MS_BIND, "")
		if !b.rw {
			must(syscall.Mount("", dst, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY|syscall.MS_NOSUID|syscall.MS_NODEV, ""), "remount ro "+dst)
		}
	}

	cmd := exec.Command("/x39jail", childArgs...)
	cmd.Dir = "/"
	cmd.Env = extraEnv
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	// Se este processo morrer, o filho morre junto (e o clean desmonta depois).
	cmd.SysProcAttr = &syscall.SysProcAttr{Chroot: jail, Pdeathsig: syscall.SIGKILL}

	sigs := make(chan os.Signal, 4)
	signal.Notify(sigs, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)
	defer signal.Stop(sigs)

	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "x39jail: %v\n", err)
		return 2
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	if runTimeout > 0 {
		time.AfterFunc(runTimeout, func() {
			fmt.Fprintln(os.Stderr, "x39jail: tempo limite, encerrando")
			cmd.Process.Signal(syscall.SIGTERM)
			time.AfterFunc(5*time.Second, func() { cmd.Process.Kill() })
		})
	}

	killed := false
	for {
		select {
		case sig := <-sigs:
			// 1º sinal: repassa ao filho; 2º: mata de vez.
			if killed {
				cmd.Process.Kill()
			} else {
				killed = true
				cmd.Process.Signal(sig)
				go func() {
					time.Sleep(5 * time.Second)
					cmd.Process.Kill()
				}()
			}
		case err := <-done:
			if err != nil {
				if ee, ok := err.(*exec.ExitError); ok {
					return ee.ExitCode()
				}
				fmt.Fprintf(os.Stderr, "x39jail: %v\n", err)
				return 2
			}
			return 0
		}
	}
}

// cleanMounts desmonta tudo o que estiver montado sob a jaula (do mais
// profundo para o mais raso) e remove o diretório. Devolve quantos desmontou.
func cleanMounts() int {
	var points []string
	if f, err := os.Open("/proc/mounts"); err == nil {
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			fields := strings.Fields(sc.Text())
			if len(fields) < 2 {
				continue
			}
			mp := strings.ReplaceAll(fields[1], "\\040", " ")
			if mp == jail || strings.HasPrefix(mp, jail+"/") {
				points = append(points, mp)
			}
		}
		f.Close()
	}
	sort.Slice(points, func(i, j int) bool { return len(points[i]) > len(points[j]) })
	n := 0
	for _, mp := range points {
		if syscall.Unmount(mp, syscall.MNT_DETACH) == nil {
			n++
		}
	}
	os.Remove(jail)
	return n
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ---------------------------------------------------------------- filho

func prctl(op, a, b uintptr) (uintptr, syscall.Errno) {
	r, _, e := syscall.RawSyscall(syscall.SYS_PRCTL, op, a, b)
	return r, e
}

// childPrepare roda como root dentro da jaula e termina como usuário comum,
// sem capabilities e sem como ganhá-las de volta.
func childPrepare() {
	runtime.LockOSThread()
	if os.Geteuid() != 0 {
		die("filho deveria começar como root dentro da jaula")
	}
	for c := uintptr(0); c <= maxCap; c++ {
		if _, e := prctl(prCapbsetDrop, c, 0); e != 0 && e != syscall.EINVAL {
			die("PR_CAPBSET_DROP %d: %v", c, e)
		}
	}
	if err := syscall.Setgroups([]int{}); err != nil {
		die("setgroups: %v", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		die("setgid: %v", err)
	}
	if err := syscall.Setuid(uid); err != nil {
		die("setuid: %v", err)
	}
	if _, e := prctl(prSetNoNewPrivs, 1, 0); e != 0 {
		die("no_new_privs: %v", e)
	}
}

func runTarget(argv []string) {
	// o pai repassa só o que veio de -env; o resto é fixo
	env := append([]string{"PATH=/app:/bin", "HOME=/data", "TMPDIR=/tmp"}, os.Environ()...)
	err := syscall.Exec(argv[0], argv, env)
	die("exec %s: %v", argv[0], err)
}

// ---------------------------------------------------------------- teste

var failures int

func check(name string, ok bool, detail string) {
	st := "PASS"
	if !ok {
		st = "FAIL"
		failures++
	}
	fmt.Printf("[%s] %s %s\n", st, name, detail)
}

func expectErr(name string, err error) {
	if err == nil {
		check(name, false, "(deveria falhar, mas funcionou)")
		return
	}
	check(name, true, "→ "+err.Error())
}

func selfTest() int {
	fmt.Printf("uid=%d euid=%d gid=%d egid=%d\n", os.Getuid(), os.Geteuid(), os.Getgid(), os.Getegid())
	check("rodando como usuário comum", os.Getuid() == uid && os.Geteuid() == uid && os.Getgid() == gid && os.Getegid() == gid, "")
	g, _ := os.Getgroups()
	check("sem grupos suplementares", len(g) == 0, fmt.Sprint(g))
	expectErr("não consegue voltar a root (setuid 0)", syscall.Setuid(0))

	nnp, _ := prctl(prGetNoNewPrivs, 0, 0)
	check("no_new_privs ligado", nnp == 1, "")
	allDropped := true
	for c := uintptr(0); c <= maxCap; c++ {
		r, e := prctl(prCapbsetRead, c, 0)
		if e == 0 && r != 0 {
			allDropped = false
			fmt.Printf("  capability %d ainda no conjunto de limite\n", c)
		}
	}
	check("todas as capabilities removidas do conjunto de limite", allDropped, "")

	for _, p := range []string{
		"/dev/nand0", "/dev/nand0p1", "/dev/nand0p2", "/dev/nand0p3", "/dev/nand0p4",
		"/dev/nand0p5", "/dev/nand0p6", "/dev/nand0p7",
		"/dev/mmcblk0", "/dev/mmcblk0p1", "/dev/mem", "/dev/kmem",
	} {
		// só leitura: se a jaula falhasse, isso apenas leria, nunca gravaria
		f, err := os.OpenFile(p, os.O_RDONLY, 0)
		if err == nil {
			f.Close()
		}
		expectErr("abrir "+p+" (leitura)", err)
	}

	expectErr("mknod de nand0 (brk 93:0)", syscall.Mknod("/dev/nand0", syscall.S_IFBLK|0600, 93<<8))
	expectErr("mknod de mem (chr 1:1)", syscall.Mknod("/dev/mem", syscall.S_IFCHR|0600, 1<<8|1))
	expectErr("mount tmpfs", syscall.Mount("tmpfs", "/dev", "tmpfs", 0, ""))
	expectErr("remount / como gravável", syscall.Mount("", "/", "", syscall.MS_REMOUNT, ""))
	expectErr("sinal para o PID 1", syscall.Kill(1, 0))

	_, err := os.Create("/arquivo-teste")
	expectErr("criar arquivo em /", err)
	_, err = os.Create("/dev/arquivo-teste")
	expectErr("criar arquivo em /dev", err)

	for _, p := range []string{"/mnt", "/var", "/bin", "/etc", "/proc", "/sys", "/usr", "/lib"} {
		_, err := os.Stat(p)
		check("não enxerga "+p, err != nil, "")
	}

	fmt.Println()
	if failures == 0 {
		fmt.Println("RESULTADO: jaula OK — sem acesso à NAND.")
		return 0
	}
	fmt.Printf("RESULTADO: %d verificação(ões) FALHARAM — NÃO use esta jaula.\n", failures)
	return 1
}
