package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// saída real do console com o teclado (Logitech) na USB-A 2 e o teclado do próprio aparelho
const devicesAmostra = `I: Bus=0019 Vendor=0001 Product=0001 Version=0100
N: Name="atc260x_onoff"
P: Phys=
S: Sysfs=/devices/platform/atc260x-onoff/input/input0
H: Handlers=kbd event0
B: EV=100003

I: Bus=0019 Vendor=0001 Product=0001 Version=0100
N: Name="atc2603c-adckeypad"
P: Phys=
S: Sysfs=/devices/platform/atc2603c-adckeypad/input/input1
H: Handlers=kbd js0 event1
B: EV=10001b

I: Bus=0003 Vendor=046d Product=c52b Version=0111
N: Name="Logitech USB Receiver"
P: Phys=usb-aotg_hcd.1-1/input0
S: Sysfs=/devices/platform/aotg_hcd.1/usb1/1-1/1-1:1.0/input/input2
H: Handlers=sysrq kbd event2
B: EV=120013

I: Bus=0003 Vendor=046d Product=c52b Version=0111
N: Name="Logitech USB Receiver"
P: Phys=usb-aotg_hcd.1-1/input1
S: Sysfs=/devices/platform/aotg_hcd.1/usb1/1-1/1-1:1.1/input/input3
H: Handlers=kbd event3
B: EV=1f0013

I: Bus=0003 Vendor=0079 Product=0011 Version=0110
N: Name="USB Gamepad"
P: Phys=usb-aotg_hcd.1-2/input0
S: Sysfs=/devices/platform/aotg_hcd.1/usb1/1-2/1-2:1.0/input/input4
H: Handlers=js1 event4
B: EV=1b
`

func TestParseInputDevices(t *testing.T) {
	devs := parseInputDevices(devicesAmostra)
	if len(devs) != 5 {
		t.Fatalf("esperava 5 dispositivos, veio %d", len(devs))
	}
	if devs[0].Host || devs[1].Host {
		t.Error("os botões do próprio aparelho não são da porta USB")
	}
	if !devs[2].Host || devs[2].Event != 2 || !devs[2].IsKeyboard || devs[2].Js != -1 {
		t.Errorf("teclado USB: %+v", devs[2])
	}
	pads := findPads(devs)
	if len(pads) != 1 || pads[0].Js != 1 || pads[0].Event != 4 || pads[0].Vendor != "0079" || pads[0].Product != "0011" {
		t.Errorf("controle: %+v", pads)
	}
	if got := findPads(devs[:4]); len(got) != 0 {
		t.Errorf("só teclado e botões do aparelho: não há controle, veio %+v", got)
	}
}

func TestAxisSign(t *testing.T) {
	r8 := absRange{0, 255} // controle genérico: centro 127
	r16 := absRange{-32768, 32767}
	hat := absRange{-1, 1}
	casos := []struct {
		v    int32
		r    absRange
		want int32
	}{
		{127, r8, 0}, {0, r8, -1}, {255, r8, 1}, {100, r8, 0}, {200, r8, 1},
		{0, r16, 0}, {-30000, r16, -1}, {30000, r16, 1}, {-5000, r16, 0},
		{0, hat, 0}, {-1, hat, -1}, {1, hat, 1},
	}
	for _, c := range casos {
		if got := axisSign(c.v, c.r); got != c.want {
			t.Errorf("axisSign(%d, %v) = %d, esperado %d", c.v, c.r, got, c.want)
		}
	}
}

func TestEmitAxisNSoltaATecla(t *testing.T) {
	ch := make(chan Event, 16)
	var last int32
	for _, v := range []int32{1, 0, 1, -1, 0} {
		emitAxisN(ch, &last, v, KeyLeft, KeyRight)
	}
	close(ch)
	want := []Event{{KeyRight, true}, {KeyRight, false}, {KeyRight, true}, {KeyRight, false}, {KeyLeft, true}, {KeyLeft, false}}
	i := 0
	for e := range ch {
		if i >= len(want) || e != want[i] {
			t.Fatalf("evento %d = %+v, esperado %+v", i, e, want[min(i, len(want)-1)])
		}
		i++
	}
	if i != len(want) {
		t.Fatalf("%d eventos, esperado %d", i, len(want))
	}
}

func TestPadCfg(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "controles"), 0o755)
	os.WriteFile(filepath.Join(d, "controles", "padrao.cfg"), []byte(
		"# comentário\ninput_player2_a_btn = \"1\"\ninput_player2_up_axis = \"-1\"\n"+
			"system_directory = \"/etc\"\n"+ // fora do filtro
			"input_player2_joypad_index = \"7\"\n"+ // o lançador manda no índice
			"input_player2_b_btn = \"2\" \n"), 0o644)
	os.WriteFile(filepath.Join(d, "controles", "0079-0011.cfg"), []byte("input_player2_a_btn = \"9\"\n"), 0o644)

	geral := padCfg(d, Pad{Vendor: "1234", Product: "5678"})
	for _, quer := range []string{"input_max_users = \"2\"", "input_player2_joypad_index = \"3\"", "input_player2_a_btn = \"1\"", "input_player2_b_btn = \"2\""} {
		if !strings.Contains(geral, quer) {
			t.Errorf("faltou %q em:\n%s", quer, geral)
		}
	}
	for _, nao := range []string{"system_directory", "joypad_index = \"7\"", "comentário"} {
		if strings.Contains(geral, nao) {
			t.Errorf("não deveria conter %q em:\n%s", nao, geral)
		}
	}
	if esp := padCfg(d, Pad{Vendor: "0079", Product: "0011"}); !strings.Contains(esp, "input_player2_a_btn = \"9\"") || strings.Contains(esp, "up_axis") {
		t.Errorf("perfil específico do controle deveria prevalecer:\n%s", esp)
	}
	if vazio := padCfg(t.TempDir(), Pad{}); !strings.Contains(vazio, "input_max_users = \"2\"") {
		t.Errorf("sem arquivo, deve ao menos ligar o jogador 2:\n%s", vazio)
	}
}

func TestPadNavMap(t *testing.T) {
	d := t.TempDir()
	os.MkdirAll(filepath.Join(d, "controles"), 0o755)
	os.WriteFile(filepath.Join(d, "controles", "081f-e401.cfg"), []byte(
		"input_player2_a_btn = \"1\"\nlancador_a = \"289\"\nlancador_b = \"290\"\nlancador_y = \"291\"\nlancador_start = \"297\"\nlancador_xx = \"5\"\n"), 0o644)
	m := padNavMap(d, "081F", "E401") // o /proc usa minúsculas, mas o arquivo vale de qualquer jeito
	if m[289] != KeyA || m[290] != KeyB || m[291] != KeyY || m[297] != KeyStart {
		t.Errorf("mapa do controle: %v", m)
	}
	if _, ok := m[288]; ok {
		t.Error("288 não foi mapeado no arquivo: não deveria valer como A do genérico")
	}
	if got := padNavMap(d, "0000", "0000"); got[304] != KeyA { // sem arquivo: mapa genérico
		t.Errorf("sem arquivo deveria valer o genérico: %v", got)
	}
}
