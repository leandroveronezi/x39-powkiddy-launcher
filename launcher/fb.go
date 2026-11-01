package main

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	fbiogetVscreeninfo = 0x4600
	fbiogetFscreeninfo = 0x4602
)

// Framebuffer escreve a tela do console (/dev/fb0, normalmente 480x272 em RGB565 com 3 páginas).
type Framebuffer struct {
	fd       int
	mem      []byte
	W, H     int
	bpp      int
	line     int
	pageSize int
	pages    int
	own      int // página em que o lançador desenha e que ele manda exibir
}

// fbOrigY é o yoffset que o firmware tinha antes de o lançador tomar a tela (devolvido ao sair).
var fbOrigY = -1

func ioctl(fd int, req uintptr, buf []byte) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(&buf[0])))
	if e != 0 {
		return e
	}
	return nil
}

func OpenFramebuffer(path string) (*Framebuffer, error) {
	fd, err := syscall.Open(path, syscall.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	var vi, fi [256]byte
	if err := ioctl(fd, fbiogetVscreeninfo, vi[:]); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	if err := ioctl(fd, fbiogetFscreeninfo, fi[:]); err != nil {
		syscall.Close(fd)
		return nil, err
	}
	le := binary.LittleEndian
	f := &Framebuffer{
		fd:   fd,
		W:    int(le.Uint32(vi[0:])),
		H:    int(le.Uint32(vi[4:])),
		bpp:  int(le.Uint32(vi[24:])),
		line: int(le.Uint32(fi[44:])),
	}
	yvirt := int(le.Uint32(vi[12:]))
	smem := int(le.Uint32(fi[20:]))
	if (f.bpp != 16 && f.bpp != 32) || f.W <= 0 || f.H <= 0 || f.line <= 0 || smem <= 0 {
		syscall.Close(fd)
		return nil, fmt.Errorf("formato de vídeo não suportado: %dx%d %dbpp", f.W, f.H, f.bpp)
	}
	f.pageSize = f.line * f.H
	f.pages = yvirt / f.H
	if f.pages < 1 {
		f.pages = 1
	}
	if fbOrigY < 0 {
		fbOrigY = int(le.Uint32(vi[20:]))
	}
	// Página própria: SEMPRE diferente da que o yoffset indica agora. Depois do RetroArch/SDL o hardware pode
	// estar apontando para outra página enquanto o driver ainda guarda o yoffset antigo; pedir "exibir a página
	// X" com X igual ao yoffset guardado é ignorado pelo driver e a tela fica preta. Com uma página diferente,
	// o primeiro pedido é uma mudança real e o driver escreve no hardware. (Medido em 24/10/2026: pintar a
	// página 2 e voltar à 1 reviveu a tela.)
	cur := int(le.Uint32(vi[20:])) / f.H
	f.own = (cur + 1) % f.pages
	mem, err := syscall.Mmap(fd, 0, smem, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}
	f.mem = mem
	return f, nil
}

// Present copia o quadro (RGB565) para a página própria do lançador e manda exibi-la. A cópia é de memória
// pura: o formato da tela e o do framebuffer são o mesmo.
func (f *Framebuffer) Present(s *Screen) {
	if f.bpp != 16 || f.line != f.W*2 || s.W != f.W || s.H != f.H {
		return
	}
	off := f.own * f.pageSize
	if off+f.pageSize > len(f.mem) {
		off = 0
	}
	src := unsafe.Slice((*byte)(unsafe.Pointer(&s.Pix[0])), len(s.Pix)*2)
	copy(f.mem[off:off+f.pageSize], src)
	f.panTo(off / f.line)
}

// Show manda exibir a página própria (barato; repete-se em repouso caso o firmware vire a página).
func (f *Framebuffer) Show() { f.panTo(f.own * f.H) }

// Restore devolve a página que o firmware exibia antes de o lançador assumir.
func (f *Framebuffer) Restore() {
	if fbOrigY >= 0 {
		f.panTo(fbOrigY)
	}
}

// panTo exibe a partir da linha y do framebuffer virtual (FBIOPAN_DISPLAY).
func (f *Framebuffer) panTo(y int) {
	var vi [256]byte
	if ioctl(f.fd, fbiogetVscreeninfo, vi[:]) != nil {
		return
	}
	binary.LittleEndian.PutUint32(vi[16:], 0)
	binary.LittleEndian.PutUint32(vi[20:], uint32(y))
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(f.fd), 0x4606, uintptr(unsafe.Pointer(&vi[0]))) // FBIOPAN_DISPLAY
}

// ---- terminal (VT): o RetroArch/SDL devolve o terminal ao modo texto ao sair; no modo texto o kernel pode
// apagar a tela por inatividade e desenhar o cursor sobre o framebuffer. Em modo gráfico isso não acontece.
var (
	ttyFd   = -1
	ttyOrig = int32(-1) // modo que o terminal tinha antes de o lançador assumir (devolvido ao sair)
)

const (
	kdGetMode = 0x4B3B
	kdSetMode = 0x4B3A
	kdText    = 0
	kdGraphic = 1
)

// TakeTTY põe o terminal em modo gráfico (repetir depois de cada jogo: o RetroArch o restaura para texto).
func TakeTTY() {
	if ttyFd < 0 {
		fd, err := syscall.Open("/dev/tty0", syscall.O_RDWR, 0)
		if err != nil {
			return
		}
		ttyFd = fd
		var m int32
		if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), kdGetMode, uintptr(unsafe.Pointer(&m))); e == 0 {
			ttyOrig = m
		}
	}
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(ttyFd), kdSetMode, kdGraphic)
}

// ReleaseTTY devolve o modo que o terminal tinha antes.
func ReleaseTTY() {
	if ttyFd < 0 {
		return
	}
	if ttyOrig >= 0 {
		syscall.Syscall(syscall.SYS_IOCTL, uintptr(ttyFd), kdSetMode, uintptr(ttyOrig))
	}
	syscall.Close(ttyFd)
	ttyFd = -1
}

// Unblank liga o vídeo caso esteja bloqueado (FBIOBLANK, FB_BLANK_UNBLANK).
func (f *Framebuffer) Unblank() {
	syscall.Syscall(syscall.SYS_IOCTL, uintptr(f.fd), 0x4611, 0)
}

// Wake desbloqueia o vídeo (FBIOBLANK) e reaplica a página visível (FBIOPAN_DISPLAY). Depois do RetroArch/SDL
// (que devolve o terminal ao modo texto) a imagem pode continuar na memória de vídeo com a tela preta.
func (f *Framebuffer) Wake() {
	f.Unblank() // a página nova só é exibida quando o lançador desenhar nela (Present)
}

func (f *Framebuffer) Close() {
	syscall.Munmap(f.mem)
	syscall.Close(f.fd)
}
