// fbtest: teste mínimo de vídeo e botões para rodar dentro do x39jail.
//
// Pinta o framebuffer (/dev/fb0) com barras de cor e borda, mostra um quadrado
// colorido a cada botão apertado e imprime os eventos no terminal. Ao terminar
// (ou após o tempo limite) restaura o conteúdo original da tela.
// Não altera o modo de vídeo, não grava arquivos e não usa rede.
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

const (
	fbiogetVscreeninfo = 0x4600
	fbiogetFscreeninfo = 0x4602
	evKey              = 1
	evAbs              = 3
)

func ioctl(fd int, req uintptr, buf []byte) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&buf[0])))
	if e != 0 {
		return e
	}
	return nil
}

func main() {
	secs := 20
	mode := "all" // info | map | paint | all
	if len(os.Args) > 2 {
		mode = os.Args[2]
	}
	if len(os.Args) > 1 {
		if n, err := strconv.Atoi(os.Args[1]); err == nil && n > 0 && n <= 120 {
			secs = n
		}
	}

	if len(mode) > 4 && mode[:4] == "dev:" {
		devTest(mode[4:], secs)
		return
	}

	fb, err := os.OpenFile("/dev/fb0", os.O_RDWR, 0)
	if err != nil {
		fmt.Println("não abriu /dev/fb0:", err)
		os.Exit(1)
	}
	defer fb.Close()

	var vi, fi [256]byte
	if err := ioctl(int(fb.Fd()), fbiogetVscreeninfo, vi[:]); err != nil {
		fmt.Println("FBIOGET_VSCREENINFO:", err)
		os.Exit(1)
	}
	if err := ioctl(int(fb.Fd()), fbiogetFscreeninfo, fi[:]); err != nil {
		fmt.Println("FBIOGET_FSCREENINFO:", err)
		os.Exit(1)
	}
	le := binary.LittleEndian
	xres, yres := int(le.Uint32(vi[0:])), int(le.Uint32(vi[4:]))
	yvirt := int(le.Uint32(vi[12:]))
	bpp := int(le.Uint32(vi[24:]))
	smemLen := int(le.Uint32(fi[20:]))
	line := int(le.Uint32(fi[44:]))
	fmt.Printf("fb0: %dx%d (virtual y=%d) %dbpp linha=%d bytes mem=%d\n", xres, yres, yvirt, bpp, line, smemLen)
	if (bpp != 16 && bpp != 32) || xres <= 0 || yres <= 0 || line <= 0 || smemLen <= 0 {
		fmt.Println("formato de vídeo não suportado por este teste")
		os.Exit(1)
	}

	if mode == "info" {
		return
	}

	mem, err := syscall.Mmap(int(fb.Fd()), 0, smemLen, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		fmt.Println("mmap fb0:", err)
		os.Exit(1)
	}
	orig := make([]byte, len(mem))
	copy(orig, mem)
	defer func() {
		copy(mem, orig)
		syscall.Munmap(mem)
		fmt.Println("tela restaurada")
	}()

	if mode == "map" {
		fmt.Println("mmap ok")
		return
	}

	pages := yvirt / yres
	if pages < 1 {
		pages = 1
	}
	px := func(r, g, b uint8) []byte {
		if bpp == 32 {
			return []byte{b, g, r, 0}
		}
		v := uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
		return []byte{byte(v), byte(v >> 8)}
	}
	rect := func(x0, y0, w, h int, c []byte) {
		for p := 0; p < pages; p++ {
			for y := y0; y < y0+h && y < yres; y++ {
				for x := x0; x < x0+w && x < xres; x++ {
					off := (p*yres+y)*line + x*len(c)
					if off >= 0 && off+len(c) <= len(mem) {
						copy(mem[off:], c)
					}
				}
			}
		}
	}

	colors := [][3]uint8{{255, 0, 0}, {0, 255, 0}, {0, 0, 255}, {255, 255, 255}, {255, 255, 0}, {0, 255, 255}, {255, 0, 255}, {0, 0, 0}}
	bw := xres / len(colors)
	for i, c := range colors {
		rect(i*bw, 0, bw, yres, px(c[0], c[1], c[2]))
	}
	white := px(255, 255, 255)
	rect(0, 0, xres, 4, white)
	rect(0, yres-4, xres, 4, white)
	rect(0, 0, 4, yres, white)
	rect(xres-4, 0, 4, yres, white)

	rect(0, 4, xres, 36, px(0, 0, 0))

	if mode == "paint" {
		time.Sleep(time.Duration(secs) * time.Second)
		return
	}

	type input struct {
		fd   int
		name string
	}
	var inputs []input
	for _, p := range []string{"/dev/input/event0", "/dev/input/event1", "/dev/input/js0"} {
		// syscalls diretas: o poller (epoll) do Go derruba a sessão neste kernel
		fd, err := syscall.Open(p, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
		if err != nil {
			fmt.Println("sem", p, "->", err)
			continue
		}
		inputs = append(inputs, input{fd, p})
		fmt.Println("lendo", p)
	}
	defer func() {
		for _, in := range inputs {
			syscall.Close(in.fd)
		}
	}()
	fmt.Printf("aperte botões por %ds...\n", secs)

	slot := 0
	deadline := time.Now().Add(time.Duration(secs) * time.Second)
	buf := make([]byte, 16)
	for time.Now().Before(deadline) {
		for _, f := range inputs {
			for {
				n, err := syscall.Read(f.fd, buf)
				if err != nil || n <= 0 {
					break
				}
				if f.name == "/dev/input/js0" {
					if n == 8 {
						fmt.Printf("%s tipo=%d num=%d valor=%d\n", f.name, buf[6], buf[7], int(int16(le.Uint16(buf[4:]))))
						if buf[6]&0x80 == 0 && int16(le.Uint16(buf[4:])) != 0 {
							rect(10+(slot%20)*24, 12, 20, 20, white)
							slot++
						}
					}
					continue
				}
				if n != 16 {
					continue
				}
				typ, code, val := le.Uint16(buf[8:]), le.Uint16(buf[10:]), int32(le.Uint32(buf[12:]))
				if typ == 0 { // EV_SYN
					continue
				}
				fmt.Printf("%s tipo=%d codigo=%d valor=%d\n", f.name, typ, code, val)
				if (typ == evKey && val == 1) || (typ == evAbs && val != 0) {
					rect(10+(slot%20)*24, 12, 20, 20, white)
					slot++
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("tempo esgotado")
}

// devTest abre só um dispositivo de entrada e lê por secs segundos (diagnóstico).
// Usa syscalls diretas (sem o poller do Go).
func devTest(path string, secs int) {
	fmt.Println("abrindo", path)
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		fmt.Println("erro:", err)
		return
	}
	defer syscall.Close(fd)
	fmt.Println("aberto; lendo", secs, "s")
	buf := make([]byte, 16)
	deadline := time.Now().Add(time.Duration(secs) * time.Second)
	for time.Now().Before(deadline) {
		n, err := syscall.Read(fd, buf)
		if n > 0 {
			fmt.Printf("%s leu %d bytes: %x\n", path, n, buf[:n])
		} else if err != nil && err != syscall.EAGAIN {
			fmt.Println("erro de leitura:", err)
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("fim")
}
