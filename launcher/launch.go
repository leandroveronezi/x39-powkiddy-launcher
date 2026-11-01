package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Config descreve onde estão as coisas no console.
//
//	Root: pasta em RAM com o programa (app/, rt/lib, x39jail, tinymix)  — ex.: /tmp/x39
//	Card: pasta do nosso cartão (cores/, roms/, data/)                  — ex.: /mnt/card/x39
type Config struct {
	Root     string
	Card     string
	GameTime int    // se > 0, encerra o jogo depois de N segundos (só para testes)
	Hook     bool   // iniciado pelo gancho de boot: oferece "Voltar ao menu original" nas Opções
	OKFile   string // se não vazio, é removido depois da primeira tela desenhada (proteção contra reinício em laço)
	Vivo     string // se não vazio, é criado depois da primeira tela (desarma o temporizador de segurança do gancho)
	Grab     bool   // toma o uso exclusivo do botão de ligar e do teclado (o msg_server do firmware deixa de ver)
}

// gameCfg monta o x39.cfg anexado à configuração do RetroArch: volume e as opções do sistema (sistema.json),
// já filtradas por validOpts. O volume vem por último e prevalece.
func gameCfg(s System, volumeDB int, extra string) string {
	var keys []string
	for k := range s.Opts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s = \"%s\"\n", k, s.Opts[k])
	}
	b.WriteString(extra)
	fmt.Fprintf(&b, "audio_volume = \"%d.000000\"\n", volumeDB)
	return b.String()
}

// jailHas pergunta à jaula se ela tem um recurso (x39jail features). Uma jaula antiga não tem o comando: falso.
func jailHas(root, feature string) bool {
	out, err := exec.Command(filepath.Join(root, "x39jail"), "features").Output()
	if err != nil {
		return false
	}
	for _, f := range strings.Fields(string(out)) {
		if f == feature {
			return true
		}
	}
	return false
}

// padJoypadIndex é o índice do SDL do controle na jaula: o SDL lista primeiro os event* (event0, event1) e depois os js*
// (js0 = console, js1 = controle USB), então o controle é o joystick 3 (medido com tools/diagnostico/sdljoy em 25/10/2026).
const padJoypadIndex = "3"

// padLineRe aceita só linhas de configuração do jogador 2 (nada de caminhos nem de outras opções).
var padLineRe = regexp.MustCompile(`^input_player2_[a-z0-9_]+ = "[^"\r\n]{0,40}"$`)

// padCfg monta as linhas do jogador 2 para um controle USB: x39/controles/<vendor>-<produto>.cfg se existir, senão
// x39/controles/padrao.cfg. Só linhas input_player2_* passam pelo filtro. Sem arquivo, só liga o jogador 2.
func padCfg(card string, p Pad) string {
	var raw []byte
	for _, n := range []string{strings.ToLower(p.Vendor) + "-" + strings.ToLower(p.Product) + ".cfg", "padrao.cfg"} {
		if b, err := os.ReadFile(filepath.Join(card, "controles", n)); err == nil {
			raw = b
			break
		}
	}
	var b strings.Builder
	b.WriteString("input_max_users = \"2\"\ninput_player2_joypad_index = \"" + padJoypadIndex + "\"\n")
	for _, ln := range strings.Split(string(raw), "\n") {
		ln = strings.TrimSpace(ln)
		if padLineRe.MatchString(ln) && !strings.HasPrefix(ln, "input_player2_joypad_index") {
			b.WriteString(ln + "\n")
		}
	}
	return b.String()
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// headsetPlugged diz se há fone no jack: /sys/class/switch/h2w/state vale 4 sem nada e 3 com o fone plugado
// (medido no X39 em 25/10/2026); 1 e 2 são os outros tipos de plugue do padrão h2w.
func headsetPlugged() bool {
	switch readTrim("/sys/class/switch/h2w/state") {
	case "1", "2", "3":
		return true
	}
	return false
}

// setSpeaker liga/desliga o alto-falante (controle 35 do mixer, "speaker on off switch").
func setSpeaker(mix string, on bool) {
	v := "0"
	if on {
		v = "1"
	}
	exec.Command(mix, "35", v).Run()
}

func writeStr(path, v string) { os.WriteFile(path, []byte(v), 0) }

// Launch roda um jogo na jaula e devolve quando ele terminar, restaurando mixer e CPU.
func (c Config) Launch(g Game, volumeDB int) error {
	s, ok := systemByID(g.SysID)
	if !ok {
		return fmt.Errorf("sistema desconhecido: %s", g.SysID)
	}
	coresRAM := filepath.Join(c.Root, "app", "cores")
	os.MkdirAll(coresRAM, 0o755)
	// a RAM do console é pequena: mantém só o core necessário
	if ents, err := os.ReadDir(coresRAM); err == nil {
		for _, e := range ents {
			if e.Name() != s.Core {
				os.Remove(filepath.Join(coresRAM, e.Name()))
			}
		}
	}
	coreDst := filepath.Join(coresRAM, s.Core)
	if _, err := os.Stat(coreDst); err != nil {
		if err := copyFile(filepath.Join(c.Card, "cores", s.Core), coreDst); err != nil {
			return fmt.Errorf("core %s: %w", s.Core, err)
		}
	}

	romDir := filepath.Join(c.Card, "roms", s.ID)
	rel, err := filepath.Rel(romDir, g.File)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("jogo fora da pasta do sistema: %s", g.File)
	}
	dataDir := filepath.Join(c.Card, "data", s.ID)
	os.MkdirAll(filepath.Join(dataDir, "system"), 0o755)
	os.MkdirAll(filepath.Join(dataDir, "saves"), 0o755)
	os.MkdirAll(filepath.Join(dataDir, "states"), 0o755)

	// ganho dos jogos escolhido nas Opções (anexado à configuração do RetroArch)
	// jogador 2: um controle na porta USB hospedeira (USB-A 2) entra na jaula como js1/event2
	var padArgs []string
	var padExtra string
	if pads := FindPads(); len(pads) > 0 && jailHas(c.Root, "input") {
		p := pads[0]
		// SÓ o nó js do controle: o SDL deste fork joga TODOS os eventos dos nós event* no joystick 0 (comentário do autor:
		// "always fire the events to device 0"), então um event* do controle faria ele comandar o jogador 1 junto com o console.
		padArgs = []string{"-input", fmt.Sprintf("/dev/input/js%d=js1", p.Js)}
		padExtra = padCfg(c.Card, p)
		fmt.Fprintf(os.Stderr, "jogador 2: %s (js%d, event%d)\n", p.Name, p.Js, p.Event)
	}
	os.WriteFile(filepath.Join(dataDir, "x39.cfg"), []byte(gameCfg(s, volumeDB, padExtra)), 0o644)

	mix := filepath.Join(c.Root, "tinymix")
	exec.Command(mix, "30", "1").Run()
	setSpeaker(mix, !headsetPlugged()) // com fone plugado o alto-falante fica desligado
	exec.Command(mix, "15", "40").Run()
	defer func() {
		exec.Command(mix, "30", "0").Run()
		exec.Command(mix, "35", "0").Run()
		exec.Command(mix, "15", "40").Run()
	}()

	// vigia o fone durante o jogo: plugar/desplugar troca o alto-falante na hora
	stopJack := make(chan struct{})
	var jackWG sync.WaitGroup
	jackWG.Add(1)
	go func() {
		defer jackWG.Done()
		last := headsetPlugged()
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-stopJack:
				return
			case <-t.C:
				if h := headsetPlugged(); h != last {
					last = h
					setSpeaker(mix, !h)
				}
			}
		}
	}()
	defer func() { close(stopJack); jackWG.Wait() }() // roda ANTES de o mixer ser restaurado (defers em ordem inversa)

	const cpu0 = "/sys/devices/system/cpu/cpu0/cpufreq"
	const cpu1 = "/sys/devices/system/cpu/cpu1/cpufreq"
	gov, max := readTrim(cpu0+"/scaling_governor"), readTrim(cpu0+"/scaling_max_freq")
	for _, d := range []string{cpu0, cpu1} {
		writeStr(d+"/scaling_max_freq", "900000")
		writeStr(d+"/scaling_governor", "performance")
	}
	defer func() {
		if gov != "" && max != "" {
			for _, d := range []string{cpu0, cpu1} {
				writeStr(d+"/scaling_governor", gov)
				writeStr(d+"/scaling_max_freq", max)
			}
		}
	}()

	logf, _ := os.Create(filepath.Join(c.Root, "run.log"))
	if logf != nil {
		defer logf.Close()
	}
	args := []string{"run", "-dev"}
	if c.GameTime > 0 {
		args = append(args, "-t", fmt.Sprint(c.GameTime))
	}
	cmd := exec.Command(filepath.Join(c.Root, "x39jail"), args...)
	cmd.Args = append(cmd.Args, padArgs...)
	cmd.Args = append(cmd.Args,
		"-env", "SDL_VIDEODRIVER=fbcon", "-env", "SDL_NOMOUSE=1", "-env", "SDL_MOUSEDEV=/dev/null",
		"-ro", filepath.Join(c.Root, "app")+"=/app",
		"-ro", filepath.Join(c.Root, "rt", "lib")+"=/lib",
		"-rw", dataDir+"=/data",
		"-ro", romDir+"=/roms",
		"--", "/app/retroarch", "-c", "/app/retroarch.cfg", "--appendconfig", "/data/x39.cfg")
	if c.GameTime > 0 {
		cmd.Args = append(cmd.Args, "-v") // modo de teste: log detalhado (inclui a taxa de áudio)
	}
	cmd.Args = append(cmd.Args, "-L", "/app/cores/"+s.Core, "/roms/"+rel)
	cmd.Stdout, cmd.Stderr = logf, logf
	if err := cmd.Start(); err != nil {
		return err
	}
	gameMu.Lock()
	gameCmd = cmd
	gameMu.Unlock()
	werr := cmd.Wait()
	gameMu.Lock()
	gameCmd = nil
	gameMu.Unlock()
	return werr
}

// gameCmd é a jaula com o jogo em andamento (nil = nenhum). StopGame a encerra se o lançador for terminado,
// para o RetroArch nunca ficar órfão (sem ninguém para devolver a tela e as teclas).
var (
	gameMu  sync.Mutex
	gameCmd *exec.Cmd
)

// StopGame pede à jaula que termine (a jaula derruba o jogo e desmonta tudo); se não sair em 3 s, mata.
func StopGame() {
	gameMu.Lock()
	c := gameCmd
	gameMu.Unlock()
	if c == nil || c.Process == nil {
		return
	}
	c.Process.Signal(syscall.SIGTERM)
	go func() {
		time.Sleep(3 * time.Second)
		c.Process.Kill()
	}()
}
