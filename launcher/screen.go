package main

// Screen é a tela em RGB565, igual ao formato do framebuffer do console: apresentar um quadro
// é só copiar memória. Todos os elementos pesados são pré-renderizados em Sprites e fundos.
type Screen struct {
	W, H int
	Pix  []uint16
}

func NewScreen(w, h int) *Screen { return &Screen{W: w, H: h, Pix: make([]uint16, w*h)} }

func pack565(r, g, b uint32) uint16 {
	return uint16((r>>3)<<11 | (g>>2)<<5 | (b >> 3))
}

var bayer4 = [16]int{0, 8, 2, 10, 12, 4, 14, 6, 3, 11, 1, 9, 15, 7, 13, 5}

// To565 converte a imagem de 32 bits para RGB565 com dithering ordenado (evita faixas nos degradês).
func (m *Img) To565() []uint16 {
	out := make([]uint16, m.W*m.H)
	for y := 0; y < m.H; y++ {
		for x := 0; x < m.W; x++ {
			p := m.Pix[y*m.W+x]
			t := bayer4[(y&3)*4+(x&3)] // 0..15
			r := min(255, int(p>>16&255)+t/2)
			g := min(255, int(p>>8&255)+t/4)
			b := min(255, int(p&255)+t/2)
			out[y*m.W+x] = pack565(uint32(r), uint32(g), uint32(b))
		}
	}
	return out
}

// Sprite é uma imagem RGB565 com máscara alfa de 8 bits.
type Sprite struct {
	W, H int
	C    []uint16
	A    []uint8
}

// blend565 mistura s sobre d com alfa a (0 a 256).
func blend565(d, s uint16, a int) uint16 {
	dr, dg, db := int(d>>11), int(d>>5&63), int(d&31)
	sr, sg, sb := int(s>>11), int(s>>5&63), int(s&31)
	r := dr + ((sr-dr)*a)>>8
	g := dg + ((sg-dg)*a)>>8
	b := db + ((sb-db)*a)>>8
	return uint16(r<<11 | g<<5 | b)
}

// Blit desenha o sprite centrado em (cx,cy), com escala (vizinho mais próximo) e alfa global (0 a 256).
func (s *Screen) Blit(cx, cy int, sp *Sprite, scale float64, alpha int) {
	if alpha <= 0 {
		return
	}
	dw, dh := int(float64(sp.W)*scale+0.5), int(float64(sp.H)*scale+0.5)
	if dw <= 0 || dh <= 0 {
		return
	}
	x0, y0 := cx-dw/2, cy-dh/2
	step := 0
	if dw != sp.W || dh != sp.H {
		step = int(65536 / scale)
	}
	// recorte horizontal
	dxa, dxb := 0, dw
	if x0 < 0 {
		dxa = -x0
	}
	if x0+dw > s.W {
		dxb = s.W - x0
	}
	if dxa >= dxb {
		return
	}
	for dy := 0; dy < dh; dy++ {
		py := y0 + dy
		if py < 0 || py >= s.H {
			continue
		}
		sy := dy
		if step != 0 {
			sy = (dy * step) >> 16
			if sy >= sp.H {
				sy = sp.H - 1
			}
		}
		srow := sy * sp.W
		drow := py * s.W
		for dx := dxa; dx < dxb; dx++ {
			sx := dx
			if step != 0 {
				sx = (dx * step) >> 16
				if sx >= sp.W {
					sx = sp.W - 1
				}
			}
			i := srow + sx
			a := int(sp.A[i])
			if a == 0 {
				continue
			}
			if alpha < 256 {
				a = a * alpha >> 8
			}
			d := drow + x0 + dx
			if a >= 255 {
				s.Pix[d] = sp.C[i]
			} else {
				s.Pix[d] = blend565(s.Pix[d], sp.C[i], a+(a>>7))
			}
		}
	}
}

// FillRect pinta um retângulo translúcido (alfa de 0 a 255).
func (s *Screen) FillRect(x, y, w, h int, col uint32, alpha int) {
	c := pack565(col>>16&255, col>>8&255, col&255)
	for iy := max(y, 0); iy < min(y+h, s.H); iy++ {
		for ix := max(x, 0); ix < min(x+w, s.W); ix++ {
			i := iy*s.W + ix
			if alpha >= 255 {
				s.Pix[i] = c
			} else {
				s.Pix[i] = blend565(s.Pix[i], c, alpha+(alpha>>7))
			}
		}
	}
}

// Text desenha o texto (topo da linha em y) misturando o alfa do glifo com o do argumento.
func (s *Screen) Text(f *Font, x, y int, str string, col uint32, alpha int) {
	c := pack565(col>>16&255, col>>8&255, col&255)
	base := y + f.Ascent
	for _, r := range str {
		g, ok := f.glyphs[r]
		if !ok {
			g = f.glyphs['?']
		}
		for iy := 0; iy < g.H; iy++ {
			py := base + g.OY + iy
			if py < 0 || py >= s.H {
				continue
			}
			for ix := 0; ix < g.W; ix++ {
				px := x + g.OX + ix
				if px < 0 || px >= s.W {
					continue
				}
				a := int(f.img.GrayAt(g.X+ix, g.Y+iy).Y)
				if a == 0 {
					continue
				}
				a = a * alpha / 255
				i := py*s.W + px
				if a >= 253 {
					s.Pix[i] = c
				} else {
					s.Pix[i] = blend565(s.Pix[i], c, a+(a>>7))
				}
			}
		}
		x += g.Adv
	}
}

func (s *Screen) TextC(f *Font, cx, y int, str string, col uint32, alpha int) {
	s.Text(f, cx-f.Width(str)/2, y, str, col, alpha)
}

// ---------------------------------------------------------------- construção de sprites

// boxBlur borra uma máscara w×h (valores 0 a 255) com caixa de raio r, em duas passadas por eixo.
func boxBlur(m []uint8, w, h, r int) []uint8 {
	a := make([]int, w*h)
	for i, v := range m {
		a[i] = int(v)
	}
	b := make([]int, w*h)
	for pass := 0; pass < 2; pass++ {
		for y := 0; y < h; y++ { // horizontal
			sum := 0
			for x := -r; x <= r; x++ {
				sum += a[y*w+clampInt(x, 0, w-1)]
			}
			for x := 0; x < w; x++ {
				b[y*w+x] = sum / (2*r + 1)
				sum += a[y*w+clampInt(x+r+1, 0, w-1)] - a[y*w+clampInt(x-r, 0, w-1)]
			}
		}
		for x := 0; x < w; x++ { // vertical
			sum := 0
			for y := -r; y <= r; y++ {
				sum += b[clampInt(y, 0, h-1)*w+x]
			}
			for y := 0; y < h; y++ {
				a[y*w+x] = sum / (2*r + 1)
				sum += b[clampInt(y+r+1, 0, h-1)*w+x] - b[clampInt(y-r, 0, h-1)*w+x]
			}
		}
	}
	out := make([]uint8, w*h)
	for i, v := range a {
		out[i] = uint8(v)
	}
	return out
}

// roundCover devolve a cobertura (0 a 255) de um retângulo arredondado w×h com raio r.
func roundCover(w, h, r int) []uint8 {
	cov := make([]uint8, w*h)
	fr := float64(r)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var dx, dy float64
			inCorner := false
			switch {
			case x < r && y < r:
				dx, dy, inCorner = fr-float64(x)-0.5, fr-float64(y)-0.5, true
			case x >= w-r && y < r:
				dx, dy, inCorner = float64(x-(w-r))+0.5, fr-float64(y)-0.5, true
			case x < r && y >= h-r:
				dx, dy, inCorner = fr-float64(x)-0.5, float64(y-(h-r))+0.5, true
			case x >= w-r && y >= h-r:
				dx, dy, inCorner = float64(x-(w-r))+0.5, float64(y-(h-r))+0.5, true
			}
			if inCorner {
				cov[y*w+x] = cornerCover(dx, dy, fr)
			} else {
				cov[y*w+x] = 255
			}
		}
	}
	return cov
}

// newRoundSprite compõe um sprite arredondado a partir de uma imagem opaca w×h, com sombra opcional.
func newRoundSprite(card *Img, r, margin, shadowAlpha int) *Sprite {
	w, h := card.W, card.H
	W, H := w+2*margin, h+2*margin
	cov := roundCover(w, h, r)
	var shadow []uint8
	if margin > 0 && shadowAlpha > 0 {
		m := make([]uint8, W*H)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				yy := y + margin + 4
				if yy < H {
					m[yy*W+x+margin] = cov[y*w+x]
				}
			}
		}
		shadow = boxBlur(m, W, H, 6)
	}
	sp := &Sprite{W: W, H: H, C: make([]uint16, W*H), A: make([]uint8, W*H)}
	for y := 0; y < H; y++ {
		for x := 0; x < W; x++ {
			ca := 0
			var rgbc uint32
			cx, cy := x-margin, y-margin
			if cx >= 0 && cy >= 0 && cx < w && cy < h {
				ca = int(cov[cy*w+cx])
				rgbc = card.Pix[cy*w+cx]
			}
			sa := 0
			if shadow != nil {
				sa = int(shadow[y*W+x]) * shadowAlpha / 255
			}
			a := ca + sa*(255-ca)/255
			if a == 0 {
				continue
			}
			// cor: o cartão sobre a sombra preta
			cr := int(rgbc>>16&255) * ca / a
			cg := int(rgbc>>8&255) * ca / a
			cb := int(rgbc&255) * ca / a
			i := y*W + x
			sp.C[i] = pack565(uint32(cr), uint32(cg), uint32(cb))
			sp.A[i] = uint8(a)
		}
	}
	return sp
}
