package main

import "math"

// Img é a tela em memória: pixels 0x00RRGGBB, linha a linha.
type Img struct {
	W, H int
	Pix  []uint32
}

func NewImg(w, h int) *Img {
	return &Img{W: w, H: h, Pix: make([]uint32, w*h)}
}

func rgb(r, g, b uint8) uint32 { return uint32(r)<<16 | uint32(g)<<8 | uint32(b) }

// mix devolve a*(255-t)+b*t (t de 0 a 255).
func mix(a, b uint32, t uint8) uint32 {
	if t == 0 {
		return a
	}
	if t == 255 {
		return b
	}
	k := uint32(t)
	ar, ag, ab := a>>16&255, a>>8&255, a&255
	br, bg, bb := b>>16&255, b>>8&255, b&255
	r := (ar*(255-k) + br*k + 127) / 255
	g := (ag*(255-k) + bg*k + 127) / 255
	bl := (ab*(255-k) + bb*k + 127) / 255
	return r<<16 | g<<8 | bl
}

func (c *Img) put(x, y int, col uint32, a uint8) {
	if x < 0 || y < 0 || x >= c.W || y >= c.H || a == 0 {
		return
	}
	i := y*c.W + x
	if a == 255 {
		c.Pix[i] = col
	} else {
		c.Pix[i] = mix(c.Pix[i], col, a)
	}
}

func (c *Img) Fill(col uint32) {
	for i := range c.Pix {
		c.Pix[i] = col
	}
}

// VGradient pinta um retângulo com degradê vertical.
func (c *Img) VGradient(x, y, w, h int, top, bot uint32) {
	for iy := 0; iy < h; iy++ {
		t := uint8(0)
		if h > 1 {
			t = uint8(iy * 255 / (h - 1))
		}
		col := mix(top, bot, t)
		py := y + iy
		if py < 0 || py >= c.H {
			continue
		}
		for ix := 0; ix < w; ix++ {
			px := x + ix
			if px >= 0 && px < c.W {
				c.Pix[py*c.W+px] = col
			}
		}
	}
}

// cobertura de um canto arredondado (0 a 255) para o pixel (px,py) relativo ao canto.
func cornerCover(dx, dy, r float64) uint8 {
	d := math.Sqrt(dx*dx + dy*dy)
	v := r - d + 0.5
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 255
	}
	return uint8(v * 255)
}

// RoundRect pinta um retângulo arredondado com degradê vertical (top→bot) e alfa global.
func (c *Img) RoundRect(x, y, w, h, r int, top, bot uint32, alpha uint8) {
	if r*2 > w {
		r = w / 2
	}
	if r*2 > h {
		r = h / 2
	}
	fr := float64(r)
	for iy := 0; iy < h; iy++ {
		py := y + iy
		if py < 0 || py >= c.H {
			continue
		}
		t := uint8(0)
		if h > 1 {
			t = uint8(iy * 255 / (h - 1))
		}
		col := mix(top, bot, t)
		for ix := 0; ix < w; ix++ {
			px := x + ix
			if px < 0 || px >= c.W {
				continue
			}
			cov := uint8(255)
			var dx, dy float64
			switch {
			case ix < r && iy < r:
				dx, dy = fr-float64(ix)-0.5, fr-float64(iy)-0.5
				cov = cornerCover(dx, dy, fr)
			case ix >= w-r && iy < r:
				dx, dy = float64(ix-(w-r))+0.5, fr-float64(iy)-0.5
				cov = cornerCover(dx, dy, fr)
			case ix < r && iy >= h-r:
				dx, dy = fr-float64(ix)-0.5, float64(iy-(h-r))+0.5
				cov = cornerCover(dx, dy, fr)
			case ix >= w-r && iy >= h-r:
				dx, dy = float64(ix-(w-r))+0.5, float64(iy-(h-r))+0.5
				cov = cornerCover(dx, dy, fr)
			}
			a := uint8(uint32(cov) * uint32(alpha) / 255)
			c.put(px, py, col, a)
		}
	}
}

// Shadow desenha uma sombra suave abaixo de um retângulo arredondado (camadas translúcidas).
func (c *Img) Shadow(x, y, w, h, r, spread int, alpha uint8) {
	for i := spread; i >= 1; i-- {
		a := uint8(int(alpha) * (spread - i + 1) / (spread*spread/2 + 4))
		c.RoundRect(x-i, y-i+spread/2+2, w+2*i, h+2*i, r+i, 0x000000, 0x000000, a)
	}
}

// Circle pinta um círculo com borda suave.
func (c *Img) Circle(cx, cy, r int, col uint32, alpha uint8) {
	fr := float64(r)
	for y := cy - r - 1; y <= cy+r+1; y++ {
		for x := cx - r - 1; x <= cx+r+1; x++ {
			d := math.Hypot(float64(x-cx), float64(y-cy))
			v := fr - d + 0.5
			if v <= 0 {
				continue
			}
			a := uint8(255)
			if v < 1 {
				a = uint8(v * 255)
			}
			c.put(x, y, col, uint8(uint32(a)*uint32(alpha)/255))
		}
	}
}

// Rect pinta um retângulo simples com alfa.
func (c *Img) Rect(x, y, w, h int, col uint32, alpha uint8) {
	for iy := 0; iy < h; iy++ {
		for ix := 0; ix < w; ix++ {
			c.put(x+ix, y+iy, col, alpha)
		}
	}
}
