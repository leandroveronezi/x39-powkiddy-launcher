// evlog: imprime os eventos de EV_ABS e EV_KEY dos dispositivos de entrada (só leitura), por N segundos,
// com o nome do dispositivo e o instante (ms) de cada evento.
// Uso: evlog <segundos> /dev/input/event0 /dev/input/event1
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

func ioctl(fd int, req uintptr, p unsafe.Pointer) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(p))
	if e != 0 {
		return e
	}
	return nil
}

func main() {
	secs, _ := strconv.Atoi(os.Args[1])
	t0 := time.Now()
	end := t0.Add(time.Duration(secs) * time.Second)
	for i, p := range os.Args[2:] {
		fd, err := syscall.Open(p, syscall.O_RDONLY, 0)
		if err != nil {
			fmt.Println("erro", p, err)
			continue
		}
		name := make([]byte, 128)
		ioctl(fd, 0x80804506, unsafe.Pointer(&name[0])) // EVIOCGNAME(128)
		n := 0
		for n < len(name) && name[n] != 0 {
			n++
		}
		fmt.Printf("dev%d = %s (%s)\n", i, p, name[:n])
		go func(fd, i int) {
			buf := make([]byte, 256)
			for {
				n, err := syscall.Read(fd, buf)
				if err != nil || n <= 0 {
					continue
				}
				ms := time.Since(t0).Milliseconds()
				for o := 0; o+16 <= n; o += 16 {
					t := binary.LittleEndian.Uint16(buf[o+8:])
					c := binary.LittleEndian.Uint16(buf[o+10:])
					v := int32(binary.LittleEndian.Uint32(buf[o+12:]))
					if t == 1 || t == 3 {
						fmt.Printf("%6dms dev%d tipo=%d codigo=%d valor=%d\n", ms, i, t, c, v)
					}
				}
			}
		}(fd, i)
	}
	time.Sleep(time.Until(end))
}
