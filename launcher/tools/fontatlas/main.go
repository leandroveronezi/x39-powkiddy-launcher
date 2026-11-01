// fontatlas: ferramenta de desenvolvimento (roda só no PC) que rasteriza uma fonte TTF
// em um atlas de glifos (PNG em tons de cinza = canal alfa) + métricas em JSON.
// O lançador embute esses arquivos e os lê só com a biblioteca padrão do Go.
//
// Uso: fontatlas <saida-dir> <nome>=<arquivo.ttf>:<px> ...
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type Glyph struct {
	X, Y, W, H int // retângulo no atlas
	OX, OY     int // deslocamento do desenho em relação ao ponto da linha de base
	Adv        int // avanço horizontal em pixels
}

type Atlas struct {
	Size    int
	Ascent  int
	Descent int
	Height  int
	Glyphs  map[string]Glyph
}

func runes() []rune {
	var r []rune
	for c := rune(32); c <= 126; c++ {
		r = append(r, c)
	}
	for c := rune(160); c <= 255; c++ {
		r = append(r, c)
	}
	// símbolos usados na interface
	r = append(r, 0x2022, 0x2026, 0x2013, 0x2014, 0x25B6, 0x25C0, 0x2605, 0x2713, 0x266B, 0x2190, 0x2191, 0x2192, 0x2193, 0x25B2, 0x25BC)
	return r
}

func gerar(outDir, nome, ttf string, px int) error {
	data, err := os.ReadFile(ttf)
	if err != nil {
		return err
	}
	f, err := opentype.Parse(data)
	if err != nil {
		return err
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{Size: float64(px), DPI: 72, Hinting: font.HintingFull})
	if err != nil {
		return err
	}
	defer face.Close()
	m := face.Metrics()
	asc := m.Ascent.Ceil()
	desc := m.Descent.Ceil()

	const atlasW = 512
	cx, cy, rowH := 1, 1, 0
	type item struct {
		r    rune
		mask *image.Alpha
		g    Glyph
	}
	var items []item
	for _, r := range runes() {
		dr, mask, maskp, adv, ok := face.Glyph(fixed.P(0, 0), r)
		if !ok {
			continue
		}
		w, h := dr.Dx(), dr.Dy()
		g := Glyph{W: w, H: h, OX: dr.Min.X, OY: dr.Min.Y, Adv: adv.Round()}
		if w > 0 && h > 0 {
			if cx+w+1 > atlasW {
				cx, cy, rowH = 1, cy+rowH+1, 0
			}
			g.X, g.Y = cx, cy
			cx += w + 1
			if h > rowH {
				rowH = h
			}
			// copia só a região do glifo
			sub := image.NewAlpha(image.Rect(0, 0, w, h))
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					_, _, _, al := mask.At(maskp.X+x, maskp.Y+y).RGBA()
					sub.SetAlpha(x, y, color.Alpha{A: uint8(al >> 8)})
				}
			}
			items = append(items, item{r, sub, g})
		} else {
			items = append(items, item{r, nil, g})
		}
	}
	atlasH := cy + rowH + 1
	img := image.NewGray(image.Rect(0, 0, atlasW, atlasH))
	a := Atlas{Size: px, Ascent: asc, Descent: desc, Height: asc + desc, Glyphs: map[string]Glyph{}}
	for _, it := range items {
		if it.mask != nil {
			for y := 0; y < it.g.H; y++ {
				for x := 0; x < it.g.W; x++ {
					img.SetGray(it.g.X+x, it.g.Y+y, color.Gray{Y: it.mask.AlphaAt(x, y).A})
				}
			}
		}
		a.Glyphs[strconv.Itoa(int(it.r))] = it.g
	}

	pf, err := os.Create(filepath.Join(outDir, nome+".png"))
	if err != nil {
		return err
	}
	if err := png.Encode(pf, img); err != nil {
		pf.Close()
		return err
	}
	pf.Close()
	jf, err := os.Create(filepath.Join(outDir, nome+".json"))
	if err != nil {
		return err
	}
	enc := json.NewEncoder(jf)
	if err := enc.Encode(a); err != nil {
		jf.Close()
		return err
	}
	jf.Close()
	fmt.Printf("%s: %dx%d, %d glifos, altura %d\n", nome, atlasW, atlasH, len(a.Glyphs), a.Height)
	return nil
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "uso: fontatlas <saida-dir> <nome>=<arquivo.ttf>:<px> ...")
		os.Exit(2)
	}
	out := os.Args[1]
	for _, spec := range os.Args[2:] {
		p := strings.SplitN(spec, "=", 2)
		q := strings.SplitN(p[1], ":", 2)
		px, err := strconv.Atoi(q[1])
		if len(p) != 2 || len(q) != 2 || err != nil {
			fmt.Fprintln(os.Stderr, "argumento inválido:", spec)
			os.Exit(2)
		}
		if err := gerar(out, p[0], q[0], px); err != nil {
			fmt.Fprintln(os.Stderr, "erro:", err)
			os.Exit(1)
		}
	}
}
