// fbinfo: imprime todos os campos de FBIOGET_VSCREENINFO e FBIOGET_FSCREENINFO do /dev/fb0 (só leitura).
// Serve para comparar o estado do vídeo "bom" com o "preto" depois do RetroArch.
package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

func main() {
	path := "/dev/fb0"
	if len(os.Args) > 1 {
		path = os.Args[1]
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY, 0)
	if err != nil {
		fmt.Println("erro:", err)
		os.Exit(1)
	}
	var vi, fi [256]byte
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x4600, uintptr(unsafe.Pointer(&vi[0]))); e != 0 {
		fmt.Println("vscreeninfo:", e)
	}
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x4602, uintptr(unsafe.Pointer(&fi[0]))); e != 0 {
		fmt.Println("fscreeninfo:", e)
	}
	le := binary.LittleEndian
	names := []string{"xres", "yres", "xres_virtual", "yres_virtual", "xoffset", "yoffset", "bits_per_pixel", "grayscale",
		"red.offset", "red.length", "red.msb_right", "green.offset", "green.length", "green.msb_right",
		"blue.offset", "blue.length", "blue.msb_right", "transp.offset", "transp.length", "transp.msb_right",
		"nonstd", "activate", "height", "width", "accel_flags", "pixclock", "left_margin", "right_margin",
		"upper_margin", "lower_margin", "hsync_len", "vsync_len", "sync", "vmode", "rotate", "colorspace"}
	fmt.Print("var:")
	for i, n := range names {
		fmt.Printf(" %s=%d", n, int32(le.Uint32(vi[i*4:])))
	}
	fmt.Println()
	fmt.Printf("fix: id=%q smem_start=0x%x smem_len=%d type=%d type_aux=%d visual=%d xpanstep=%d ypanstep=%d ywrapstep=%d line_length=%d\n",
		string(fi[:16]), le.Uint32(fi[16:]), le.Uint32(fi[20:]), le.Uint32(fi[24:]), le.Uint32(fi[28:]), le.Uint32(fi[32:]),
		le.Uint16(fi[36:]), le.Uint16(fi[38:]), le.Uint16(fi[40:]), le.Uint32(fi[44:]))
}
