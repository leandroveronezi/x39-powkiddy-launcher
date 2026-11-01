package main

import (
	"syscall"
	"time"
	"unsafe"
)

// Códigos do driver de watchdog do Linux (linux/watchdog.h).
const (
	wdiocSetOptions = 0x80045704
	wdiocKeepalive  = 0x80045705
	wdiocSetTimeout = 0xC0045706
	wdiosEnableCard = 2
)

// Watchdog alimenta o watchdog de hardware do X39. No firmware original quem faz isso é o
// manager; quando o nosso lançador ocupa o lugar dele, sem este alimentador o console
// reiniciaria sozinho. Só é usado com -watchdog (iniciado pelo gancho de boot).
type Watchdog struct {
	fd   int
	stop chan struct{}
	done chan struct{}
}

func wdIoctl(fd int, req uintptr, v *int32) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(unsafe.Pointer(v)))
	if e != 0 {
		return e
	}
	return nil
}

// StartWatchdog abre /dev/watchdog, define o prazo e começa a alimentá-lo a cada 5 s.
// Devolve nil se o dispositivo não puder ser aberto.
func StartWatchdog(timeoutSecs int) *Watchdog {
	fd, err := syscall.Open("/dev/watchdog", syscall.O_WRONLY, 0)
	if err != nil {
		return nil
	}
	t := int32(timeoutSecs)
	wdIoctl(fd, wdiocSetTimeout, &t)
	opt := int32(wdiosEnableCard)
	wdIoctl(fd, wdiocSetOptions, &opt)
	w := &Watchdog{fd: fd, stop: make(chan struct{}), done: make(chan struct{})}
	feed := func() {
		var dummy int32
		if wdIoctl(fd, wdiocKeepalive, &dummy) != nil {
			syscall.Write(fd, []byte{0})
		}
	}
	feed()
	go func() {
		defer close(w.done)
		tk := time.NewTicker(5 * time.Second)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				feed()
			case <-w.stop:
				return
			}
		}
	}()
	return w
}

// Stop desarma o watchdog de forma limpa (caractere mágico 'V'), para o menu original assumir.
func (w *Watchdog) Stop() {
	if w == nil {
		return
	}
	close(w.stop)
	<-w.done
	syscall.Write(w.fd, []byte("V"))
	syscall.Close(w.fd)
}
