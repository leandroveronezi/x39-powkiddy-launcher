package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func escreve(t *testing.T, dir, nome, conteudo string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, nome)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, nome), []byte(conteudo), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSystemsValidos(t *testing.T) {
	d := t.TempDir()
	escreve(t, d, "sms/sistema.json", `{"nome":"Master System","sigla":"SMS","core":"genesis_plus_gx_libretro.so",
		"extensoes":["sms",".SG","zip"],"cor1":"#E8344A","cor2":"7A1226","ordem":10,
		"retroarch":{"video_smooth":true,"aspect_ratio_index":"22","audio_max_timing_skew":0.02}}`)
	escreve(t, d, "atari2600/sistema.json", `{"nome":"Atari 2600","core":"stella2014_libretro.so","extensoes":["a26","bin"]}`)
	sys, warn := LoadSystems(d)
	if len(warn) != 0 {
		t.Fatalf("avisos inesperados: %v", warn)
	}
	if len(sys) != 2 || sys[0].ID != "sms" || sys[1].ID != "atari2600" {
		t.Fatalf("ordem/quantidade errada: %+v", sys)
	}
	s := sys[0]
	if s.C1 != 0xE8344A || s.C2 != 0x7A1226 || s.Short != "SMS" {
		t.Errorf("cores/sigla: %+v", s)
	}
	if got := strings.Join(s.Exts, ","); got != "sms,sg,zip" {
		t.Errorf("extensões normalizadas = %q", got)
	}
	if s.Opts["video_smooth"] != "true" || s.Opts["aspect_ratio_index"] != "22" || s.Opts["audio_max_timing_skew"] != "0.02" {
		t.Errorf("opções: %v", s.Opts)
	}
	a := sys[1] // sem sigla/cores: padrões
	if a.Short != "ATARI 2600"[:5] || a.C1 != 0x6B8AAE {
		t.Errorf("padrões: %+v", a)
	}
}

func TestLoadSystemsInvalidosSaoIgnorados(t *testing.T) {
	d := t.TempDir()
	escreve(t, d, "ok/sistema.json", `{"core":"a.so","extensoes":["x"]}`)
	escreve(t, d, "semjson/leiame.txt", "nada")
	escreve(t, d, "quebrado/sistema.json", `{ não é json`)
	escreve(t, d, "semcore/sistema.json", `{"extensoes":["x"]}`)
	escreve(t, d, "corcaminho/sistema.json", `{"core":"../../etc/x.so","extensoes":["x"]}`)
	escreve(t, d, "semext/sistema.json", `{"core":"a.so","extensoes":[]}`)
	escreve(t, d, "nome ruim/sistema.json", `{"core":"a.so","extensoes":["x"]}`)
	sys, warn := LoadSystems(d)
	if len(sys) != 1 || sys[0].ID != "ok" {
		t.Fatalf("só 'ok' deveria carregar: %+v", sys)
	}
	if len(warn) != 6 {
		t.Errorf("esperava 6 avisos, veio %d: %v", len(warn), warn)
	}
}

func TestOpcoesProibidas(t *testing.T) {
	out, warn := validOpts(map[string]any{
		"video_smooth":            true,
		"system_directory":        "/etc", // caminho: proibido
		"audio_driver":            "alsa", // driver: proibido
		"video_filter":            "x\ny = 1",
		"core_options_path":       "/x",
		"BAD KEY":                 "1",
		"input_player1_a_btn":     "3",
		"netplay_public_announce": "true",
		"lista":                   []any{1},
	})
	if len(out) != 2 || out["video_smooth"] != "true" || out["input_player1_a_btn"] != "3" {
		t.Errorf("deveria sobrar só 2 opções: %v", out)
	}
	if len(warn) != 7 {
		t.Errorf("avisos = %d: %v", len(warn), warn)
	}
}

func TestGameCfg(t *testing.T) {
	s := System{Opts: map[string]string{"b_opt": "2", "a_opt": "1"}}
	got := gameCfg(s, -8, "")
	want := "a_opt = \"1\"\nb_opt = \"2\"\naudio_volume = \"-8.000000\"\n"
	if got != want {
		t.Errorf("gameCfg =\n%q\nesperado\n%q", got, want)
	}
}

func TestImagemDoConsole(t *testing.T) {
	d := t.TempDir()
	// PNG 40x20: metade esquerda vermelha opaca, metade direita transparente
	im := image.NewNRGBA(image.Rect(0, 0, 40, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			im.SetNRGBA(x, y, color.NRGBA{255, 0, 0, 255})
		}
	}
	f, _ := os.Create(filepath.Join(d, "console.png"))
	png.Encode(f, im)
	f.Close()
	pic, err := loadImage(filepath.Join(d, "console.png"))
	if err != nil {
		t.Fatal(err)
	}
	c := NewImg(40, 40)
	c.Fill(0x000000)
	c.DrawFit(pic, 0, 0, 40, 40) // 40x20 em 40x40: escala 1, centrada na vertical (y de 10 a 30)
	if got := c.Pix[15*40+5]; got != 0xFF0000 {
		t.Errorf("pixel vermelho esperado em (5,15), veio %06X", got)
	}
	if got := c.Pix[15*40+30]; got != 0x000000 {
		t.Errorf("metade transparente deveria manter o fundo, veio %06X", got)
	}
	if got := c.Pix[2*40+5]; got != 0x000000 {
		t.Errorf("fora da imagem deveria manter o fundo, veio %06X", got)
	}
}

func TestImagemInvalida(t *testing.T) {
	d := t.TempDir()
	escreve(t, d, "ruim.png", "isto não é uma imagem")
	if _, err := loadImage(filepath.Join(d, "ruim.png")); err == nil {
		t.Error("deveria recusar arquivo que não é imagem")
	}
	if _, err := loadImage(filepath.Join(d, "nao-existe.png")); err == nil {
		t.Error("deveria recusar arquivo inexistente")
	}
}
