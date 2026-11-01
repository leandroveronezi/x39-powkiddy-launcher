package main

import (
	"encoding/binary"
	"syscall"
)

type Key int

const (
	KeyNone Key = iota
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyA
	KeyB
	KeyX
	KeyY
	KeyMenu
	KeyStart
	KeySelect
	KeyL1
	KeyR1
	KeyL2
	KeyR2
	KeyVolUp
	KeyVolDown
	KeyPower
)

type Event struct {
	Key  Key
	Down bool
}

// Códigos de tecla do hardware do X39 (medidos em evdev; ver README).
var keyMap = map[uint16]Key{
	352: KeyA, 158: KeyB, 308: KeyX, 139: KeyY, 174: KeyMenu,
	315: KeyStart, 314: KeySelect,
	412: KeyL1, 407: KeyR1, 312: KeyL2, 313: KeyR2,
	115: KeyVolUp, 114: KeyVolDown, 116: KeyPower,
	// cruz direcional (teclas do módulo de matriz); o joystick analógico vem como eixos (abaixo)
	103: KeyUp, 108: KeyDown, 105: KeyLeft, 106: KeyRight,
}

// inputFds guarda o descritor de cada dispositivo aberto por StartInput (para GrabInput).
var inputFds = map[string]int{}

// GrabInput toma (on) ou solta (!on) o uso EXCLUSIVO de um dispositivo de entrada (EVIOCGRAB).
// Preso, só o nosso lançador recebe os botões: o msg_server do firmware, que também lê o botão de ligar
// e o teclado, deixa de reagir a eles (era ele que suspendia/desligava o aparelho durante os testes).
// O grab some sozinho quando o processo termina.
func GrabInput(path string, on bool) {
	fd, ok := inputFds[path]
	if !ok {
		return
	}
	v := uintptr(0)
	if on {
		v = 1
	}
	const eviocgrab = 0x40044590 // _IOW('E', 0x90, int)
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), eviocgrab, v)
}

// StartInput lê os dois dispositivos de entrada do console. Usa syscall.Open/Read direto:
// o poller (epoll) do Go sobre /dev/input derruba a sessão neste kernel.
func StartInput(paths ...string) chan Event {
	ch := make(chan Event, 64)
	for _, p := range paths {
		fd, err := syscall.Open(p, syscall.O_RDONLY, 0)
		if err != nil {
			continue
		}
		inputFds[p] = fd
		go func(fd int) {
			buf := make([]byte, 16*16)
			var ax, ay int32 // último estado dos eixos do direcional
			le := binary.LittleEndian
			for {
				n, err := syscall.Read(fd, buf)
				if err != nil || n <= 0 {
					if err == syscall.EINTR || err == syscall.EAGAIN {
						continue
					}
					return
				}
				for o := 0; o+16 <= n; o += 16 {
					typ := le.Uint16(buf[o+8:])
					code := le.Uint16(buf[o+10:])
					val := int32(le.Uint32(buf[o+12:]))
					switch typ {
					case 1: // EV_KEY
						if k, ok := keyMap[code]; ok && val != 2 {
							ch <- Event{k, val == 1}
						}
					case 3: // EV_ABS: direcional como eixos
						if code == 0 {
							emitAxis(ch, &ax, val, KeyLeft, KeyRight)
						} else if code == 1 {
							emitAxis(ch, &ay, val, KeyUp, KeyDown)
						}
					}
				}
			}
		}(fd)
	}
	return ch
}

func emitAxis(ch chan<- Event, last *int32, val int32, neg, pos Key) {
	sign := func(v int32) int32 {
		switch {
		case v < -32:
			return -1
		case v > 32:
			return 1
		}
		return 0
	}
	n, o := sign(val), *last // *last já guarda o estado (-1, 0, 1): não passar por sign() de novo
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
