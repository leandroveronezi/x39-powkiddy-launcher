package main

import "testing"

func TestValidInputPath(t *testing.T) {
	ok := []string{"/dev/input/js1", "/dev/input/event12", "/dev/input/js0", "/dev/input/event999"}
	ruins := []string{"/dev/nand0", "/dev/mem", "/dev/input/mouse0", "/dev/input/js", "/dev/input/event1000",
		"/dev/input/../mem", "/dev/input/js1/x", "dev/input/js1", "/dev/input/js1 ", "/dev/input/ js1", "/dev/mmcblk0"}
	for _, p := range ok {
		if !validInputPath(p) {
			t.Errorf("%q deveria ser aceito", p)
		}
	}
	for _, p := range ruins {
		if validInputPath(p) {
			t.Errorf("%q NÃO deveria ser aceito", p)
		}
	}
}

func TestParseInputArg(t *testing.T) {
	casos := []struct {
		in, src, name string
		ok            bool
	}{
		{"/dev/input/js3", "/dev/input/js3", "js3", true},
		{"/dev/input/js3=js1", "/dev/input/js3", "js1", true},
		{"/dev/input/event7=event2", "/dev/input/event7", "event2", true},
		{"/dev/input/js3=event2", "", "", false}, // tipo diferente
		{"/dev/input/js3=nand0", "", "", false},  // nome fora do padrão
		{"/dev/input/js3=../js1", "", "", false}, // sem caminho no nome
		{"/dev/mem=js1", "", "", false},
		{"/dev/input/mouse0", "", "", false},
	}
	for _, c := range casos {
		src, name, ok := parseInputArg(c.in)
		if ok != c.ok || src != c.src || name != c.name {
			t.Errorf("parseInputArg(%q) = %q, %q, %v; esperado %q, %q, %v", c.in, src, name, ok, c.src, c.name, c.ok)
		}
	}
}
