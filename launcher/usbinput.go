package main

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Dispositivos de entrada da porta USB hospedeira (USB-A 2). Tudo o que entra por ali aparece sob o controlador
// aotg_hcd em /proc/bus/input/devices; o teclado e o botão de ligar do próprio aparelho não.

// InputDev é um bloco de /proc/bus/input/devices.
type InputDev struct {
	Name         string
	Vendor, Prod string
	Sysfs        string
	Event, Js    int // -1 se não tiver
	Host         bool
	IsKeyboard   bool
	IsMouseOnly  bool
}

var (
	eventRe = regexp.MustCompile(`\bevent(\d+)\b`)
	jsRe    = regexp.MustCompile(`\bjs(\d+)\b`)
)

// parseInputDevices lê o texto de /proc/bus/input/devices.
func parseInputDevices(txt string) []InputDev {
	var out []InputDev
	for _, blk := range strings.Split(txt, "\n\n") {
		d := InputDev{Event: -1, Js: -1}
		for _, ln := range strings.Split(blk, "\n") {
			switch {
			case strings.HasPrefix(ln, "I: "):
				for _, f := range strings.Fields(ln[3:]) {
					if strings.HasPrefix(f, "Vendor=") {
						d.Vendor = strings.TrimPrefix(f, "Vendor=")
					} else if strings.HasPrefix(f, "Product=") {
						d.Prod = strings.TrimPrefix(f, "Product=")
					}
				}
			case strings.HasPrefix(ln, "N: Name="):
				d.Name = strings.Trim(strings.TrimPrefix(ln, "N: Name="), "\"")
			case strings.HasPrefix(ln, "S: Sysfs="):
				d.Sysfs = strings.TrimPrefix(ln, "S: Sysfs=")
				d.Host = strings.Contains(d.Sysfs, "aotg_hcd")
			case strings.HasPrefix(ln, "H: Handlers="):
				h := ln[len("H: Handlers="):]
				if m := eventRe.FindStringSubmatch(h); m != nil {
					d.Event, _ = strconv.Atoi(m[1])
				}
				if m := jsRe.FindStringSubmatch(h); m != nil {
					d.Js, _ = strconv.Atoi(m[1])
				}
				d.IsKeyboard = strings.Contains(h, "kbd")
				d.IsMouseOnly = strings.Contains(h, "mouse") && !d.IsKeyboard
			}
		}
		if d.Name != "" {
			out = append(out, d)
		}
	}
	return out
}

// Pad é um controle de jogo (tem js) ligado à porta hospedeira.
type Pad struct {
	Name, Vendor, Product string
	Js, Event             int
}

// findPads devolve os controles da porta hospedeira, do menor js para o maior.
func findPads(devs []InputDev) []Pad {
	var out []Pad
	for _, d := range devs {
		if d.Host && d.Js >= 0 && d.Event >= 0 {
			out = append(out, Pad{Name: d.Name, Vendor: d.Vendor, Product: d.Prod, Js: d.Js, Event: d.Event})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Js < out[j].Js })
	return out
}

// FindPads lê o console e devolve os controles USB de agora.
func FindPads() []Pad {
	b, err := os.ReadFile("/proc/bus/input/devices")
	if err != nil {
		return nil
	}
	return findPads(parseInputDevices(string(b)))
}

// ---------------------------------------------------------------- teclado e controle como botões do lançador

// Teclado: setas, Enter/Espaço = A, Esc/Backspace = B, Y = favorito, F1/Tab = Menu, [ ] = L1 R1, PgUp/PgDn = L2 R2.
var kbdMap = map[uint16]Key{
	103: KeyUp, 108: KeyDown, 105: KeyLeft, 106: KeyRight,
	28: KeyA, 57: KeyA, 96: KeyA,
	1: KeyB, 14: KeyB,
	21: KeyY, 59: KeyMenu, 15: KeyMenu,
	26: KeyL1, 27: KeyR1, 104: KeyL2, 109: KeyR2,
	31: KeyStart, 50: KeySelect, // S, M
}

// Controle USB comum: duas famílias de numeração (BTN_SOUTH... e BTN_TRIGGER...). Para navegar no menu basta
// confirmar/voltar; o mapeamento fino de jogo fica em x39/controles/*.cfg.
var padMap = map[uint16]Key{
	304: KeyA, 288: KeyA, 289: KeyA, // confirmar
	305: KeyB, 290: KeyB, 291: KeyB, // voltar
	307: KeyY, 308: KeyX, 306: KeyX,
	310: KeyL1, 311: KeyR1, 312: KeyL2, 313: KeyR2,
	314: KeySelect, 315: KeyStart, 316: KeyMenu,
	// direcional digital em alguns controles
	544: KeyUp, 545: KeyDown, 546: KeyLeft, 547: KeyRight,
}

var navKeys = map[string]Key{
	"a": KeyA, "b": KeyB, "x": KeyX, "y": KeyY, "l1": KeyL1, "r1": KeyR1, "l2": KeyL2, "r2": KeyR2,
	"select": KeySelect, "start": KeyStart, "menu": KeyMenu,
}

var navLineRe = regexp.MustCompile(`^lancador_([a-z0-9]+) = "([0-9]{1,4})"$`)

// padNavMap devolve o mapa de botões do MENU de um controle: as linhas lancador_<tecla> = "<código evdev>" de
// x39/controles/<vendor>-<produto>.cfg. Sem o arquivo (ou sem essas linhas), vale o mapa genérico padMap.
func padNavMap(card, vendor, product string) map[uint16]Key {
	b, err := os.ReadFile(filepath.Join(card, "controles", strings.ToLower(vendor)+"-"+strings.ToLower(product)+".cfg"))
	if err != nil {
		return padMap
	}
	m := map[uint16]Key{}
	for _, ln := range strings.Split(string(b), "\n") {
		if g := navLineRe.FindStringSubmatch(strings.TrimSpace(ln)); g != nil {
			if k, ok := navKeys[g[1]]; ok {
				if code, err := strconv.Atoi(g[2]); err == nil {
					m[uint16(code)] = k
				}
			}
		}
	}
	if len(m) == 0 {
		return padMap
	}
	// o direcional digital (quando o controle o manda como botão) continua valendo
	for c, k := range padMap {
		if k == KeyUp || k == KeyDown || k == KeyLeft || k == KeyRight {
			m[c] = k
		}
	}
	return m
}

type absRange struct{ min, max int32 }

// readAbs lê o intervalo de um eixo (EVIOCGABS); falha = intervalo padrão de 0 a 255.
func readAbs(fd int, code uint16) absRange {
	var info [6]int32 // value, minimum, maximum, fuzz, flat, resolution
	req := uintptr(0x80184540) + uintptr(code)
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&info[0]))); e != 0 || info[2] <= info[1] {
		return absRange{0, 255}
	}
	return absRange{info[1], info[2]}
}

// axisSign classifica o valor de um eixo em -1, 0, +1 em relação ao centro do intervalo.
func axisSign(v int32, r absRange) int32 {
	mid := (r.min + r.max) / 2
	dead := (r.max - r.min) / 4
	switch {
	case v < mid-dead:
		return -1
	case v > mid+dead:
		return 1
	}
	return 0
}

// emitAxisN é como emitAxis, mas para qualquer intervalo de eixo e guardando o estado já classificado.
func emitAxisN(ch chan<- Event, last *int32, n int32, neg, pos Key) {
	o := *last
	if n == o {
		return
	}
	if o == -1 {
		ch <- Event{neg, false}
	} else if o == 1 {
		ch <- Event{pos, false}
	}
	if n == -1 {
		ch <- Event{neg, true}
	} else if n == 1 {
		ch <- Event{pos, true}
	}
	*last = n
}

var (
	hotMu   sync.Mutex
	hotOpen = map[int]bool{} // event nodes já em leitura
)

// StartHotplug vigia a porta USB hospedeira: a cada 2 s abre os dispositivos de entrada novos (teclado, controle)
// e lê os botões deles como se fossem do console. Plugar e desplugar a qualquer momento funciona.
func StartHotplug(ch chan Event, card string) {
	go func() {
		for {
			b, err := os.ReadFile("/proc/bus/input/devices")
			if err == nil {
				for _, d := range parseInputDevices(string(b)) {
					if !d.Host || d.Event < 0 || d.IsMouseOnly {
						continue
					}
					hotMu.Lock()
					seen := hotOpen[d.Event]
					if !seen {
						hotOpen[d.Event] = true
					}
					hotMu.Unlock()
					if !seen {
						go readHostDevice(d, ch, card)
					}
				}
			}
			time.Sleep(2 * time.Second)
		}
	}()
}

func readHostDevice(d InputDev, ch chan<- Event, card string) {
	defer func() {
		hotMu.Lock()
		delete(hotOpen, d.Event)
		hotMu.Unlock()
	}()
	fd, err := syscall.Open("/dev/input/event"+strconv.Itoa(d.Event), syscall.O_RDONLY, 0)
	if err != nil {
		return
	}
	defer syscall.Close(fd)
	isPad := d.Js >= 0
	buttons := kbdMap
	if isPad {
		buttons = padNavMap(card, d.Vendor, d.Prod)
	}
	var rx, ry, rh0x, rh0y absRange
	if isPad {
		rx, ry, rh0x, rh0y = readAbs(fd, 0), readAbs(fd, 1), readAbs(fd, 16), readAbs(fd, 17)
	}
	var ax, ay, hx, hy int32
	buf := make([]byte, 16*16)
	le := binary.LittleEndian
	for {
		n, err := syscall.Read(fd, buf)
		if err != nil || n <= 0 {
			if err == syscall.EINTR || err == syscall.EAGAIN {
				continue
			}
			return // dispositivo desplugado
		}
		for o := 0; o+16 <= n; o += 16 {
			typ, code, val := le.Uint16(buf[o+8:]), le.Uint16(buf[o+10:]), int32(le.Uint32(buf[o+12:]))
			switch typ {
			case 1: // EV_KEY
				if val == 2 {
					continue
				}
				if k, ok := buttons[code]; ok {
					ch <- Event{k, val == 1}
				}
			case 3: // EV_ABS (só controle)
				if !isPad {
					continue
				}
				switch code {
				case 0:
					emitAxisN(ch, &ax, axisSign(val, rx), KeyLeft, KeyRight)
				case 1:
					emitAxisN(ch, &ay, axisSign(val, ry), KeyUp, KeyDown)
				case 16:
					emitAxisN(ch, &hx, axisSign(val, rh0x), KeyLeft, KeyRight)
				case 17:
					emitAxisN(ch, &hy, axisSign(val, rh0y), KeyUp, KeyDown)
				}
			}
		}
	}
}
