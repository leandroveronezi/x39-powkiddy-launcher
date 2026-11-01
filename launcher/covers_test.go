package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fazPNG(t *testing.T, path string, w, h int) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	im := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			im.SetNRGBA(x, y, color.NRGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	png.Encode(f, im)
	f.Close()
}

func TestPickCover(t *testing.T) {
	imgs := []string{"/r/Zelda.JPG", "/r/Mario.png", "/r/capa.jpeg"}
	if got := pickCover("/r/Mario.nes", imgs, false); got != "/r/Mario.png" {
		t.Errorf("mesmo nome: %q", got)
	}
	if got := pickCover("/r/Outro.nes", imgs, false); got != "" {
		t.Errorf("pasta compartilhada, sem mesmo nome: não escolhe capa, veio %q", got)
	}
	if got := pickCover("/r/Outro.nes", imgs, true); got != "/r/Zelda.JPG" {
		t.Errorf("pasta do próprio jogo: primeira imagem, veio %q", got)
	}
	if got := pickCover("/r/x.nes", nil, true); got != "" {
		t.Errorf("sem imagens: %q", got)
	}
}

func TestScanComCapas(t *testing.T) {
	d := t.TempDir()
	roms := filepath.Join(d, "roms", "sms")
	// jogo em pasta própria, capa com o mesmo nome (em outro formato de caixa)
	os.MkdirAll(filepath.Join(roms, "Alex Kidd"), 0o755)
	os.WriteFile(filepath.Join(roms, "Alex Kidd", "Alex Kidd.sms"), []byte("x"), 0o644)
	fazPNG(t, filepath.Join(roms, "Alex Kidd", "Alex Kidd.png"), 20, 30)
	// jogo em pasta própria, imagem com outro nome: usa-a
	os.MkdirAll(filepath.Join(roms, "Sonic"), 0o755)
	os.WriteFile(filepath.Join(roms, "Sonic", "Sonic.sms"), []byte("x"), 0o644)
	fazPNG(t, filepath.Join(roms, "Sonic", "box.png"), 20, 30)
	// jogos soltos: só vale a imagem de mesmo nome
	os.WriteFile(filepath.Join(roms, "Rastan.sms"), []byte("x"), 0o644)
	fazPNG(t, filepath.Join(roms, "Rastan.png"), 20, 30)
	os.WriteFile(filepath.Join(roms, "Shinobi.sms"), []byte("x"), 0o644)
	// a imagem não é jogo
	sys := []System{{ID: "sms", Name: "SMS", Exts: []string{"sms"}}}
	out := Scan(filepath.Join(d, "roms"), sys)
	if len(out) != 1 || len(out[0].Games) != 4 {
		t.Fatalf("esperava 4 jogos: %+v", out)
	}
	cover := map[string]string{}
	for _, g := range out[0].Games {
		cover[g.Name] = filepath.Base(g.Cover)
	}
	want := map[string]string{"Alex Kidd": "Alex Kidd.png", "Sonic": "box.png", "Rastan": "Rastan.png", "Shinobi": "."}
	for k, v := range want {
		if cover[k] != v {
			t.Errorf("capa de %s = %q, esperado %q", k, cover[k], v)
		}
	}
}

func TestCoverCache(t *testing.T) {
	d := t.TempDir()
	c := newCoverCache()
	// cria mais capas do que o cache guarda
	var paths []string
	for i := 0; i < coverMax+3; i++ {
		p := filepath.Join(d, string(rune('a'+i))+".png")
		fazPNG(t, p, 100, 200)
		paths = append(paths, p)
	}
	for _, p := range paths {
		c.Want(p)
		select {
		case <-c.notify:
		case <-time.After(3 * time.Second):
			t.Fatal("a capa não ficou pronta")
		}
	}
	if sp, ok := c.Get(paths[len(paths)-1]); !ok || sp == nil {
		t.Error("a capa mais recente deveria estar no cache")
	}
	if _, ok := c.Get(paths[0]); ok {
		t.Error("a capa mais antiga deveria ter sido descartada")
	}
	// arquivo que não é imagem: falha uma vez e não tenta de novo
	ruim := filepath.Join(d, "ruim.png")
	os.WriteFile(ruim, []byte("nada"), 0o644)
	c.Want(ruim)
	select {
	case <-c.notify:
	case <-time.After(3 * time.Second):
		t.Fatal("falha deveria avisar")
	}
	if sp, ok := c.Get(ruim); !ok || sp != nil {
		t.Errorf("falha deveria ficar registrada como nil: %v %v", sp, ok)
	}
}

func TestBuildCoverProporcao(t *testing.T) {
	d := t.TempDir()
	p := filepath.Join(d, "larga.png")
	fazPNG(t, p, 400, 200) // 2:1
	sp := buildCover(p)
	if sp == nil {
		t.Fatal("deveria montar o sprite")
	}
	if p2 := filepath.Join(d, "miudinha.png"); true {
		fazPNG(t, p2, 4, 4) // pequena demais para ser capa
		if buildCover(p2) != nil {
			t.Error("imagem minúscula deveria ser recusada")
		}
	}
}
