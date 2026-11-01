// fbwake: diagnóstico do vídeo do X39 depois do RetroArch (imagem na memória, tela preta).
//   fbwake              desbloqueia (FBIOBLANK) e reaplica a página atual
//   fbwake pan 2 1      exibe a página 2 e depois a 1 (0,4 s entre elas): força o driver a reprogramar o hardware
//   fbwake paint 2 FF0000 [seg]   pinta a página 2 de vermelho, exibe-a por [seg] s (padrão 4) e volta à página anterior
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

func main() {
	fd, err := syscall.Open("/dev/fb0", syscall.O_RDWR, 0)
	if err != nil {
		fmt.Println("erro:", err)
		os.Exit(1)
	}
	var vi [256]byte
	call := func(req, arg uintptr) syscall.Errno {
		_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, arg)
		return e
	}
	if e := call(0x4600, uintptr(unsafe.Pointer(&vi[0]))); e != 0 { // FBIOGET_VSCREENINFO
		fmt.Println("FBIOGET_VSCREENINFO:", e)
		os.Exit(1)
	}
	h := binary.LittleEndian.Uint32(vi[4:]) // altura visível
	fmt.Printf("antes: yoffset=%d (altura %d)\n", binary.LittleEndian.Uint32(vi[20:]), h)
	if len(os.Args) > 3 && os.Args[1] == "paint" {
		n, _ := strconv.Atoi(os.Args[2])
		rgb, _ := strconv.ParseUint(os.Args[3], 16, 32)
		secs := 4
		if len(os.Args) > 4 {
			secs, _ = strconv.Atoi(os.Args[4])
		}
		pageSize := int(h) * 480 * 2
		mem, err := syscall.Mmap(fd, 0, pageSize*3, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
		if err != nil {
			fmt.Println("mmap:", err)
			os.Exit(1)
		}
		r, g, b := uint16(rgb>>16&0xff), uint16(rgb>>8&0xff), uint16(rgb&0xff)
		px := (r>>3)<<11 | (g>>2)<<5 | (b >> 3)
		for i := n * pageSize; i < (n+1)*pageSize; i += 2 {
			mem[i], mem[i+1] = byte(px), byte(px>>8)
		}
		old := binary.LittleEndian.Uint32(vi[20:])
		binary.LittleEndian.PutUint32(vi[16:], 0)
		binary.LittleEndian.PutUint32(vi[20:], uint32(n)*h)
		fmt.Printf("pintei a página %d e exibo por %d s: PAN -> %v\n", n, secs, call(0x4606, uintptr(unsafe.Pointer(&vi[0]))))
		time.Sleep(time.Duration(secs) * time.Second)
		binary.LittleEndian.PutUint32(vi[20:], old)
		fmt.Printf("volto ao yoffset %d: PAN -> %v\n", old, call(0x4606, uintptr(unsafe.Pointer(&vi[0]))))
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "pan" {
		for _, a := range os.Args[2:] {
			n, _ := strconv.Atoi(a)
			binary.LittleEndian.PutUint32(vi[16:], 0)
			binary.LittleEndian.PutUint32(vi[20:], uint32(n)*h)
			fmt.Printf("FBIOPAN_DISPLAY página %d -> %v\n", n, call(0x4606, uintptr(unsafe.Pointer(&vi[0]))))
			time.Sleep(400 * time.Millisecond)
		}
		return
	}
	fmt.Println("FBIOBLANK(0) ->", call(0x4611, 0))
	fmt.Println("FBIOPAN_DISPLAY ->", call(0x4606, uintptr(unsafe.Pointer(&vi[0]))))
}
