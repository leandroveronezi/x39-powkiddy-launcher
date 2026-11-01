// fbcount: conta quantas vezes por segundo a imagem do framebuffer MUDA (só leitura). Serve para medir
// quantos quadros distintos um jogo realmente entrega à tela (jogos com animação contínua, como o Enduro).
// Cada quadro é copiado para a memória de vídeo em ~1 ms; contamos uma mudança só depois de a imagem
// ficar estável por 2 ms, para não contar um quadro copiado em duas partes como dois.
// Uso: fbcount <segundos>
package main

import (
	"fmt"
	"os"
	"strconv"
	"syscall"
	"time"
	"unsafe"
)

func main() {
	secs := 10
	if len(os.Args) > 1 {
		secs, _ = strconv.Atoi(os.Args[1])
	}
	fd, err := syscall.Open("/dev/fb0", syscall.O_RDONLY, 0)
	if err != nil {
		fmt.Println("erro:", err)
		os.Exit(1)
	}
	const size = 480 * 272 * 2 * 3
	mem, err := syscall.Mmap(fd, 0, size, syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		fmt.Println("erro mmap:", err)
		os.Exit(1)
	}
	words := unsafe.Slice((*uint32)(unsafe.Pointer(&mem[0])), size/4)
	sum := func() uint32 { // amostra 1 palavra a cada 17 (primo): barato e sensível a qualquer movimento
		var h uint32
		for i := 0; i < len(words); i += 17 {
			h = h*31 + words[i]
		}
		return h
	}
	const settle = 2 * time.Millisecond
	end := time.Now().Add(time.Duration(secs) * time.Second)
	last := sum()
	changed := false
	var lastChange time.Time
	frames := 0
	perSec := make([]int, secs+2)
	t0 := time.Now()
	for time.Now().Before(end) {
		h := sum()
		now := time.Now()
		if h != last {
			last = h
			changed = true
			lastChange = now
		} else if changed && now.Sub(lastChange) >= settle {
			changed = false
			frames++
			perSec[int(now.Sub(t0).Seconds())]++
		}
	}
	fmt.Printf("%d mudanças de imagem em %d s = %.1f por segundo\n", frames, secs, float64(frames)/float64(secs))
	fmt.Print("por segundo:")
	for i := 0; i < secs; i++ {
		fmt.Printf(" %d", perSec[i])
	}
	fmt.Println()
}
