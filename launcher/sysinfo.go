package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ---------------------------------------------------------------- bateria

// Battery lê a carga do sistema (-1 se não houver bateria legível).
func Battery() (pct int, charging bool) {
	pct = -1
	dirs, _ := filepath.Glob("/sys/class/power_supply/*")
	for _, d := range dirs {
		t := readTrim(filepath.Join(d, "type"))
		if t != "" && !strings.EqualFold(t, "Battery") {
			continue
		}
		if v, err := strconv.Atoi(readTrim(filepath.Join(d, "capacity"))); err == nil && v >= 0 && v <= 100 {
			st := strings.ToLower(readTrim(filepath.Join(d, "status")))
			return v, st == "charging" || st == "full"
		}
	}
	return
}

// ---------------------------------------------------------------- luz da tela

type Backlight struct {
	dir string
	max int
}

func FindBacklight() *Backlight {
	dirs, _ := filepath.Glob("/sys/class/backlight/*")
	for _, d := range dirs {
		if m, err := strconv.Atoi(readTrim(filepath.Join(d, "max_brightness"))); err == nil && m > 0 {
			return &Backlight{dir: d, max: m}
		}
	}
	return nil
}

// Percent devolve o brilho atual em % (0 a 100).
func (b *Backlight) Percent() int {
	v, err := strconv.Atoi(readTrim(filepath.Join(b.dir, "brightness")))
	if err != nil {
		return 100
	}
	return v * 100 / b.max
}

// SetPercent ajusta o brilho (mínimo de 5% para a tela nunca ficar escura demais).
func (b *Backlight) SetPercent(p int) {
	p = clampInt(p, 5, 100)
	writeStr(filepath.Join(b.dir, "brightness"), strconv.Itoa(p*b.max/100))
}

// Raw lê e restaura o valor bruto de brilho (evita erro de arredondamento ao sair).
func (b *Backlight) Raw() string { return readTrim(filepath.Join(b.dir, "brightness")) }

func (b *Backlight) SetRaw(v string) {
	if v != "" {
		writeStr(filepath.Join(b.dir, "brightness"), v)
	}
}

// Power liga ou apaga a luz (bl_power: 0 = ligada, 1 = apagada).
func (b *Backlight) Power(on bool) {
	v := "1"
	if on {
		v = "0"
	}
	writeStr(filepath.Join(b.dir, "bl_power"), v)
}

// ---------------------------------------------------------------- memória e cartão

func MemFreeMB() int {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return -1
	}
	var free, cached int
	for _, l := range strings.Split(string(b), "\n") {
		f := strings.Fields(l)
		if len(f) < 2 {
			continue
		}
		switch f[0] {
		case "MemFree:":
			free, _ = strconv.Atoi(f[1])
		case "Cached:":
			cached, _ = strconv.Atoi(f[1])
		}
	}
	return (free + cached) / 1024
}

func DiskFreeMB(path string) int {
	var st syscall.Statfs_t
	if syscall.Statfs(path, &st) != nil {
		return -1
	}
	return int(int64(st.Bavail) * int64(st.Bsize) / (1 << 20))
}

// ---------------------------------------------------------------- desligar / reiniciar

func syncDisks() { syscall.Sync() }

func run(names ...string) error {
	var last error
	for _, n := range names {
		if err := exec.Command(n).Run(); err == nil {
			return nil
		} else {
			last = err
		}
	}
	return last
}

func PowerOff() error {
	syncDisks()
	return run("poweroff", "/sbin/poweroff", "/bin/poweroff")
}

func Reboot() error {
	syncDisks()
	return run("reboot", "/sbin/reboot", "/bin/reboot")
}

func fmtMB(mb int) string {
	switch {
	case mb < 0:
		return "?"
	case mb >= 1024:
		return fmt.Sprintf("%.1f GB", float64(mb)/1024)
	}
	return fmt.Sprintf("%d MB", mb)
}
