package main

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // registra o decodificador JPEG
	_ "image/png"  // registra o decodificador PNG
	"os"
)

const (
	imgMaxBytes  = 6 << 20 // arquivos maiores que isso são ignorados
	imgMaxPixels = 2048    // lado máximo aceito (evita estourar a memória ao decodificar)
)

// loadImage lê um PNG ou JPEG do cartão e o devolve como NRGBA (canal alfa preservado).
func loadImage(path string) (*image.NRGBA, error) {
	st, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if st.Size() > imgMaxBytes {
		return nil, fmt.Errorf("imagem grande demais (%d KB)", st.Size()/1024)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if cfg.Width > imgMaxPixels || cfg.Height > imgMaxPixels || cfg.Width < 1 || cfg.Height < 1 {
		return nil, fmt.Errorf("dimensões não aceitas: %dx%d", cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	b := src.Bounds()
	nr := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(nr, nr.Bounds(), src, b.Min, draw.Src)
	return nr, nil
}

// DrawFit desenha src dentro do retângulo (x, y, w, h) mantendo a proporção e centralizando. Reduz por média de
// área (sem serrilhado) e respeita a transparência da imagem. Imagens pequenas não são ampliadas além de 2x.
func (c *Img) DrawFit(src *image.NRGBA, x, y, w, h int) {
	sw, sh := src.Bounds().Dx(), src.Bounds().Dy()
	if sw < 1 || sh < 1 || w < 1 || h < 1 {
		return
	}
	scale := float64(w) / float64(sw)
	if s2 := float64(h) / float64(sh); s2 < scale {
		scale = s2
	}
	if scale > 2 {
		scale = 2
	}
	dw, dh := int(float64(sw)*scale+0.5), int(float64(sh)*scale+0.5)
	if dw < 1 || dh < 1 {
		return
	}
	ox, oy := x+(w-dw)/2, y+(h-dh)/2
	for dy := 0; dy < dh; dy++ {
		sy0 := int(float64(dy) / scale)
		sy1 := int(float64(dy+1) / scale)
		if sy1 <= sy0 {
			sy1 = sy0 + 1
		}
		if sy1 > sh {
			sy1 = sh
		}
		for dx := 0; dx < dw; dx++ {
			sx0 := int(float64(dx) / scale)
			sx1 := int(float64(dx+1) / scale)
			if sx1 <= sx0 {
				sx1 = sx0 + 1
			}
			if sx1 > sw {
				sx1 = sw
			}
			var r, g, b, a, n uint32
			for sy := sy0; sy < sy1; sy++ {
				row := src.Pix[sy*src.Stride:]
				for sx := sx0; sx < sx1; sx++ {
					p := row[sx*4 : sx*4+4]
					pa := uint32(p[3])
					r += uint32(p[0]) * pa // soma com peso do alfa: bordas transparentes não escurecem a cor
					g += uint32(p[1]) * pa
					b += uint32(p[2]) * pa
					a += pa
					n++
				}
			}
			if n == 0 || a == 0 {
				continue
			}
			col := rgb(uint8(r/a), uint8(g/a), uint8(b/a))
			c.put(ox+dx, oy+dy, col, uint8(a/n))
		}
	}
}
