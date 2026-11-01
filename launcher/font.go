package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"image"
	"image/png"
	"strconv"
	"unicode/utf8"
)

//go:embed assets/*.png assets/*.json
var assetsFS embed.FS

type glyph struct {
	X, Y, W, H int
	OX, OY     int
	Adv        int
}

type atlasJSON struct {
	Size    int
	Ascent  int
	Descent int
	Height  int
	Glyphs  map[string]glyph
}

// Font é uma fonte pré-rasterizada (atlas de alfa + métricas), de um tamanho só.
type Font struct {
	img     *image.Gray
	glyphs  map[rune]glyph
	Ascent  int
	Descent int
	Height  int
}

func loadFont(name string) *Font {
	pb, err := assetsFS.ReadFile("assets/" + name + ".png")
	if err != nil {
		panic(err)
	}
	jb, err := assetsFS.ReadFile("assets/" + name + ".json")
	if err != nil {
		panic(err)
	}
	img, err := png.Decode(bytes.NewReader(pb))
	if err != nil {
		panic(err)
	}
	g, ok := img.(*image.Gray)
	if !ok {
		panic("atlas de fonte não é cinza: " + name)
	}
	var a atlasJSON
	if err := json.Unmarshal(jb, &a); err != nil {
		panic(err)
	}
	f := &Font{img: g, glyphs: map[rune]glyph{}, Ascent: a.Ascent, Descent: a.Descent, Height: a.Height}
	for k, v := range a.Glyphs {
		n, _ := strconv.Atoi(k)
		f.glyphs[rune(n)] = v
	}
	return f
}

// Width mede o texto em pixels.
func (f *Font) Width(s string) int {
	w := 0
	for _, r := range s {
		g, ok := f.glyphs[r]
		if !ok {
			g = f.glyphs['?']
		}
		w += g.Adv
	}
	return w
}

// Fit corta o texto com reticências para caber em maxW pixels.
func (f *Font) Fit(s string, maxW int) string {
	if f.Width(s) <= maxW {
		return s
	}
	ell := "…"
	for len(s) > 0 {
		_, sz := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-sz]
		if f.Width(s+ell) <= maxW {
			return s + ell
		}
	}
	return ell
}

// Text desenha o texto com o topo da linha em y.
func (c *Img) Text(f *Font, x, y int, s string, col uint32, alpha uint8) {
	base := y + f.Ascent
	for _, r := range s {
		g, ok := f.glyphs[r]
		if !ok {
			g = f.glyphs['?']
		}
		for iy := 0; iy < g.H; iy++ {
			for ix := 0; ix < g.W; ix++ {
				a := uint32(f.img.GrayAt(g.X+ix, g.Y+iy).Y)
				if a == 0 {
					continue
				}
				c.put(x+g.OX+ix, base+g.OY+iy, col, uint8(a*uint32(alpha)/255))
			}
		}
		x += g.Adv
	}
}

// TextC desenha o texto centralizado em cx.
func (c *Img) TextC(f *Font, cx, y int, s string, col uint32, alpha uint8) {
	c.Text(f, cx-f.Width(s)/2, y, s, col, alpha)
}
