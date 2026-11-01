package main

import (
	"fmt"
	"math"
	"os"
	"time"
)

const (
	screenW = 480
	screenH = 272

	colText  = 0xF4F6FB
	colMuted = 0x9AA3C7
	colBgTop = 0x0D1024
	colBgBot = 0x1C1244
	colGold  = 0xFBBF24

	cardSize = 140 // lado do cartão de sistema (o sprite tem 14 px de margem para a sombra)
	listX    = 164 // coluna da lista de jogos
	listW    = 292
	listTop  = 44
	rowH     = 27
	listRows = 7
)

type Game struct {
	Name  string // nome para mostrar
	File  string // caminho do arquivo
	Size  int64
	SysID string // sistema real do jogo
	Key   string // "sistema/arquivo": identifica o jogo nos favoritos e recentes
	Cover string // imagem de capa (jpg/jpeg/png) ao lado do jogo; "" = sem capa
}

type Sys struct {
	System
	Games []Game
}

// Modos de tela.
const (
	modeCarousel = 0
	modeList     = 1
	modeOptions  = 2
)

// OptItem é uma linha da tela de Opções.
type OptItem struct {
	Label  string
	Value  string  // texto à direita (opcional)
	Slider bool    // mostra uma barra de 0 a 1
	Frac   float64 // preenchimento da barra
}

type OptionsView struct {
	Items   []OptItem
	Sel     int
	Lines   []string // se não vazio, mostra estas linhas (tela "Informações") em vez dos itens
	Confirm string   // se não vazio, pergunta isto (A confirma, B cancela)
}

// UI guarda o estado das telas. Tudo que é caro (fundos, cartões, sombras, barras) é
// pré-renderizado em NewUI; cada quadro só copia memória e mistura alguns sprites.
type UI struct {
	covers *coverCache // capas dos jogos (carregadas em segundo plano)
	scr    *Screen
	sys    []Sys
	st     *State
	sel    int     // sistema escolhido (target módulo n)
	target int     // alvo do carrossel, sem dar a volta (pos anima até ele)
	pos    float64 // posição animada do carrossel (índice fracionário, sem dar a volta)
	mode   int
	gsel   int
	goff   float64 // deslocamento animado da lista (índice fracionário do primeiro item)
	opt    OptionsView

	battery  int // -1 = sem leitura
	charging bool

	card   []*Sprite
	bar    []*Sprite
	panel  *Sprite
	bgCar  [][]uint16
	bgList [][]uint16
	fBig   *Font
	fName  *Font
	fBody  *Font
	fSm    *Font
	fRow   *Font
	fHint  *Font

	toast      string
	toastUntil time.Time
	toastSpr   *Sprite
}

func clampf(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func mod(a, n int) int { return ((a % n) + n) % n }

// wrapd leva d para o intervalo [-n/2, n/2] (distância circular).
func wrapd(d float64, n int) float64 {
	fn := float64(n)
	return d - fn*math.Round(d/fn)
}

func humanSize(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	case n > 0:
		return fmt.Sprintf("%d B", n)
	}
	return ""
}

// ---------------------------------------------------------------- construção (uma vez, na partida)

type hint struct{ key, label string }

func NewUI(sys []Sys, st *State, progress func(done, total int)) *UI {
	u := &UI{
		covers:  newCoverCache(),
		scr:     NewScreen(screenW, screenH),
		sys:     sys,
		st:      st,
		battery: -1,
		fBig:    loadFont("bold34"),
		fName:   loadFont("bold22"),
		fBody:   loadFont("reg16"),
		fSm:     loadFont("reg14"),
		fRow:    loadFont("cond16"),
		fHint:   loadFont("bold16"),
	}
	n := len(sys)
	u.card = make([]*Sprite, n)
	u.bar = make([]*Sprite, n)
	u.bgCar = make([][]uint16, n)
	u.bgList = make([][]uint16, n)
	for i := range sys {
		u.buildSystem(i)
		if progress != nil {
			progress(i+1, n)
		}
	}
	u.panel = u.buildPanel()
	return u
}

// buildSystem (re)cria o cartão, a barra e os dois fundos de um sistema. É chamado na partida e
// quando muda a contagem de jogos de Recentes ou Favoritos.
func (u *UI) buildSystem(i int) {
	s := &u.sys[i]
	if u.card[i] == nil {
		u.card[i] = u.buildCard(s)
		u.bar[i] = u.buildBar(s)
	}
	base := u.baseBackground(s)

	car := NewImg(screenW, screenH)
	copy(car.Pix, base.Pix)
	u.drawHeader(car, "X39")
	car.TextC(u.fName, screenW/2, 186, s.Name, colText, 255)
	cnt := fmt.Sprintf("%d jogos", len(s.Games))
	switch len(s.Games) {
	case 0:
		cnt = "vazio"
	case 1:
		cnt = "1 jogo"
	}
	car.TextC(u.fSm, screenW/2, 216, cnt, colMuted, 255)
	u.drawFooter(car, []hint{{"A", "Abrir"}, {"◀ ▶", "Trocar"}, {"MENU", "Opções"}})
	u.bgCar[i] = car.To565()

	lst := NewImg(screenW, screenH)
	copy(lst.Pix, base.Pix)
	u.drawHeader(lst, "◀  "+s.Name)
	lst.TextC(u.fSm, 86, 160, fmt.Sprintf("%d jogos", len(s.Games)), colMuted, 255)
	u.drawFooter(lst, []hint{{"A", "Jogar"}, {"B", "Voltar"}, {"Y", "Favorito"}, {"▲ ▼", "Escolher"}})
	u.bgList[i] = lst.To565()
}

// SetVirtual atualiza as listas de Recentes (posição 0) e Favoritos (posição 1).
func (u *UI) SetVirtual(recent, fav []Game) {
	u.sys[0].Games = recent
	u.sys[1].Games = fav
	u.buildSystem(0)
	u.buildSystem(1)
}

// baseBackground: degradê escuro com brilho suave na cor do sistema.
func (u *UI) baseBackground(s *Sys) *Img {
	b := NewImg(screenW, screenH)
	b.VGradient(0, 0, screenW, screenH, colBgTop, colBgBot)
	glow := func(cx, cy int, rad float64, maxA float64) {
		r2 := rad * rad
		for y := int(float64(cy) - rad); y <= int(float64(cy)+rad); y++ {
			if y < 0 || y >= screenH {
				continue
			}
			for x := int(float64(cx) - rad); x <= int(float64(cx)+rad); x++ {
				if x < 0 || x >= screenW {
					continue
				}
				dx, dy := float64(x-cx), float64(y-cy)
				d2 := (dx*dx + dy*dy) / r2
				if d2 >= 1 {
					continue
				}
				k := 1 - d2
				b.put(x, y, s.C1, uint8(k*k*maxA))
			}
		}
	}
	glow(60, 20, 230, 70)
	glow(430, 250, 190, 45)
	return b
}

func (u *UI) drawHeader(c *Img, left string) {
	c.Rect(0, 0, screenW, 34, 0x000000, 70)
	c.Rect(0, 34, screenW, 1, 0xFFFFFF, 18)
	c.Text(u.fHint, 16, 7, left, colText, 255)
}

func (u *UI) drawFooter(c *Img, hs []hint) {
	c.Rect(0, screenH-30, screenW, 30, 0x000000, 80)
	c.Rect(0, screenH-30, screenW, 1, 0xFFFFFF, 18)
	x := 16
	for _, h := range hs {
		w := u.fHint.Width(h.key)
		r := 10
		if w+10 > 2*r {
			r = (w + 10) / 2
		}
		cy := screenH - 15
		c.RoundRect(x, cy-10, 2*r, 20, 10, 0xE9ECF8, 0xB4BAD6, 255)
		c.Text(u.fHint, x+r-w/2, cy-u.fHint.Height/2+1, h.key, 0x1A1D33, 255)
		x += 2*r + 8
		c.Text(u.fSm, x, cy-u.fSm.Height/2+1, h.label, colText, 235)
		x += u.fSm.Width(h.label) + 18
	}
}

func (u *UI) buildCard(s *Sys) *Sprite {
	size := cardSize
	r := size / 7
	img := NewImg(size, size)
	img.VGradient(0, 0, size, size, s.C1, s.C2)
	img.RoundRect(3, 3, size-6, size/2-4, r-2, 0xFFFFFF, 0xFFFFFF, 34) // brilho no topo
	if s.Image != "" {
		if pic, err := loadImage(s.Image); err == nil {
			// imagem do console no alto, sigla pequena embaixo
			img.DrawFit(pic, 12, 10, size-24, size-44)
			img.TextC(u.fName, size/2, size-u.fName.Height-8, s.Short, 0xFFFFFF, 240)
			return newRoundSprite(img, r, 14, 150)
		} else {
			fmt.Fprintf(os.Stderr, "imagem do console %s: %v\n", s.Image, err)
		}
	}
	for i := 0; i < 3; i++ { // pontinhos retrô
		for j := 0; j < 3; j++ {
			img.Circle(size-14-j*7, size-14-i*7, 2, 0xFFFFFF, 40)
		}
	}
	f := u.fBig
	if f.Width(s.Short) > size-16 {
		f = u.fName
	}
	img.TextC(f, size/2, size/2-f.Height/2, s.Short, 0xFFFFFF, 255)
	return newRoundSprite(img, r, 14, 150)
}

func (u *UI) buildBar(s *Sys) *Sprite {
	img := NewImg(listW, rowH-3)
	img.VGradient(0, 0, listW, rowH-3, s.C1, s.C2)
	return newRoundSprite(img, 8, 0, 0)
}

func (u *UI) buildPanel() *Sprite {
	img := NewImg(360, 226)
	img.VGradient(0, 0, 360, 226, 0x2B3052, 0x171A30)
	return newRoundSprite(img, 14, 12, 160)
}

// ---------------------------------------------------------------- mensagem rápida

func (u *UI) Toast(msg string, d time.Duration, isErr bool) {
	w := u.fBody.Width(msg) + 40
	if w > screenW-30 {
		w = screenW - 30
	}
	img := NewImg(w, 44)
	top, bot := uint32(0x2B3052), uint32(0x1A1D36)
	if isErr {
		top, bot = 0x8A2030, 0x55101C
	}
	img.VGradient(0, 0, w, 44, top, bot)
	img.TextC(u.fBody, w/2, 12, u.fBody.Fit(msg, w-24), colText, 255)
	u.toast, u.toastUntil = msg, time.Now().Add(d)
	u.toastSpr = newRoundSprite(img, 12, 10, 150)
}

func (u *UI) ClearToast() { u.toast = "" }

// ---------------------------------------------------------------- desenho (a cada quadro)

func (u *UI) Draw() {
	if u.mode == modeList {
		u.drawList()
	} else {
		u.drawCarousel() // as Opções aparecem por cima do carrossel
		if u.mode == modeOptions {
			u.drawOptions()
		}
	}
	if u.toast != "" && time.Now().Before(u.toastUntil) {
		u.scr.Blit(screenW/2, 134, u.toastSpr, 1, 256)
	}
}

// drawStatus desenha, à direita do cabeçalho, o contador e a bateria.
func (u *UI) drawStatus(counter string) {
	s := u.scr
	right := screenW - 14
	if u.battery >= 0 {
		txt := fmt.Sprintf("%d%%", u.battery)
		tw := u.fSm.Width(txt)
		s.Text(u.fSm, right-tw, 9, txt, colText, 235)
		right -= tw + 6
		// ícone: corpo 22x11 com ponta
		x, y := right-24, 12
		col := uint32(0x6EE7B7)
		if u.charging {
			col = 0x7DD3FC
		} else if u.battery < 15 {
			col = 0xF87171
		} else if u.battery < 30 {
			col = 0xFBBF24
		}
		s.FillRect(x, y, 22, 1, 0xFFFFFF, 200)
		s.FillRect(x, y+10, 22, 1, 0xFFFFFF, 200)
		s.FillRect(x, y, 1, 11, 0xFFFFFF, 200)
		s.FillRect(x+21, y, 1, 11, 0xFFFFFF, 200)
		s.FillRect(x+22, y+3, 2, 5, 0xFFFFFF, 200)
		s.FillRect(x+2, y+2, 18*u.battery/100, 7, col, 255)
		right = x - 14
	}
	if counter != "" {
		s.Text(u.fSm, right-u.fSm.Width(counter), 9, counter, colMuted, 255)
	}
}

func (u *UI) drawCarousel() {
	n := len(u.sys)
	idx := mod(int(math.Round(u.pos)), n)
	copy(u.scr.Pix, u.bgCar[idx])
	u.drawStatus(fmt.Sprintf("%d / %d", idx+1, n))

	type item struct {
		i int
		d float64
	}
	var items []item
	for i := range u.sys {
		d := wrapd(float64(i)-u.pos, n)
		if math.Abs(d) < 2.2 {
			items = append(items, item{i, d})
		}
	}
	// do mais distante ao mais próximo, para o central ficar por cima
	for a := 0; a < len(items); a++ {
		for b := a + 1; b < len(items); b++ {
			if math.Abs(items[b].d) > math.Abs(items[a].d) {
				items[a], items[b] = items[b], items[a]
			}
		}
	}
	for _, it := range items {
		ad := clampf(math.Abs(it.d), 0, 1.6)
		scale := 1 - 0.24*math.Min(ad, 1)
		alpha := int(256 - 120*ad/1.6)
		u.scr.Blit(screenW/2+int(it.d*150), 108, u.card[it.i], scale, alpha)
	}
}

func (u *UI) drawList() {
	s := &u.sys[u.sel]
	copy(u.scr.Pix, u.bgList[u.sel])
	u.drawStatus(fmt.Sprintf("%d / %d", min(u.gsel+1, len(s.Games)), len(s.Games)))

	shown := false
	if u.gsel >= 0 && u.gsel < len(s.Games) && s.Games[u.gsel].Cover != "" && u.covers != nil {
		if sp, ok := u.covers.Get(s.Games[u.gsel].Cover); ok && sp != nil {
			u.scr.Blit(86, 100, sp, 1, 256) // capa do jogo selecionado (pronta)
			shown = true
		}
	}
	if !shown {
		u.scr.Blit(86, 98, u.card[u.sel], 0.74, 256)
	}
	if u.gsel >= 0 && u.gsel < len(s.Games) {
		g := s.Games[u.gsel]
		u.scr.TextC(u.fSm, 86, 182, humanSize(g.Size), colMuted, 255)
		if u.sel < 2 { // em Recentes e Favoritos, diz de que sistema é o jogo
			if gs, ok := systemByID(g.SysID); ok {
				u.scr.TextC(u.fSm, 86, 202, gs.Name, colMuted, 255)
			}
		}
	}
	if len(s.Games) == 0 {
		msg := "Nenhum jogo ainda."
		if u.sel == 0 {
			msg = "Os jogos que você abrir aparecem aqui."
		} else if u.sel == 1 {
			msg = "Aperte Y num jogo para favoritar."
		}
		u.scr.TextC(u.fBody, listX+listW/2, 120, msg, colMuted, 255)
		return
	}

	first := int(math.Floor(u.goff))
	frac := u.goff - float64(first)
	for r := -1; r <= listRows; r++ {
		i := first + r
		if i < 0 || i >= len(s.Games) {
			continue
		}
		y := listTop + int((float64(r)-frac)*rowH)
		if y < listTop-rowH || y > listTop+listRows*rowH {
			continue
		}
		// esmaece nas pontas da lista
		edge := 255
		if y < listTop {
			edge = 255 + (y-listTop)*255/rowH
		} else if y > listTop+(listRows-1)*rowH {
			edge = 255 - (y-(listTop+(listRows-1)*rowH))*255/rowH
		}
		edge = clampInt(edge, 0, 255)
		g := s.Games[i]
		fav := u.st != nil && u.st.IsFav(g.Key)
		maxW := listW - 24
		if fav {
			maxW -= 18
		}
		name := u.fRow.Fit(g.Name, maxW)
		if i == u.gsel {
			u.scr.Blit(listX+listW/2, y+(rowH-3)/2, u.bar[u.sel], 1, edge+(edge>>7))
			u.scr.Text(u.fRow, listX+12, y+2, name, 0xFFFFFF, edge)
		} else {
			u.scr.Text(u.fRow, listX+12, y+2, name, colText, edge*200/255)
		}
		if fav {
			u.scr.Text(u.fSm, listX+listW-22, y+3, "★", colGold, edge)
		}
	}
	if len(s.Games) > listRows {
		trackH := listRows * rowH
		u.scr.FillRect(listX+listW+8, listTop, 4, trackH, 0xFFFFFF, 28)
		th := trackH * listRows / len(s.Games)
		if th < 18 {
			th = 18
		}
		ty := listTop + int(float64(trackH-th)*u.goff/float64(len(s.Games)-listRows))
		u.scr.FillRect(listX+listW+8, ty, 4, th, 0xFFFFFF, 190)
	}
}

// drawOptions desenha o painel de Opções por cima do que já está na tela.
func (u *UI) drawOptions() {
	u.scr.FillRect(0, 0, screenW, screenH, 0x000000, 150)
	px, py := screenW/2, 136
	u.scr.Blit(px, py, u.panel, 1, 256)
	x0, top := px-180+20, py-113+12
	u.scr.Text(u.fName, x0, top, "Opções", colText, 255)
	v := &u.opt
	y := top + 36
	switch {
	case v.Confirm != "":
		u.scr.TextC(u.fBody, px, y+18, v.Confirm, colText, 255)
		u.scr.TextC(u.fSm, px, y+56, "A confirma     B cancela", colMuted, 255)
	case len(v.Lines) > 0:
		for _, l := range v.Lines {
			u.scr.Text(u.fBody, x0, y, l, colText, 240)
			y += 24
		}
		u.scr.Text(u.fSm, x0, top+186, "B volta", colMuted, 255)
	default:
		for i, it := range v.Items {
			ry := y + i*27
			if i == v.Sel {
				u.scr.FillRect(px-170, ry-2, 340, 25, 0xFFFFFF, 34)
			}
			col := uint32(colText)
			if i != v.Sel {
				col = 0xC8CEE8
			}
			u.scr.Text(u.fBody, x0, ry, it.Label, col, 255)
			if it.Slider {
				bx, bw := px+10, 90
				u.scr.FillRect(bx, ry+8, bw, 6, 0xFFFFFF, 50)
				u.scr.FillRect(bx, ry+8, int(float64(bw)*clampf(it.Frac, 0, 1)), 6, 0x6EE7B7, 255)
				if it.Value != "" {
					u.scr.Text(u.fSm, bx+bw+8, ry+3, it.Value, colMuted, 255)
				}
			} else if it.Value != "" {
				u.scr.Text(u.fSm, px+170-14-u.fSm.Width(it.Value), ry+3, it.Value, colMuted, 255)
			}
		}
	}
}

// Step avança as animações um quadro e diz se ainda há movimento.
func (u *UI) Step() bool {
	moving := false
	d := float64(u.target) - u.pos
	if math.Abs(d) < 0.004 {
		u.pos = float64(u.target)
	} else {
		u.pos += d * 0.32
		moving = true
	}
	if u.mode == modeList {
		s := &u.sys[u.sel]
		want := clampf(float64(u.gsel)-3, 0, math.Max(0, float64(len(s.Games)-listRows)))
		d := want - u.goff
		if math.Abs(d) < 0.01 {
			u.goff = want
		} else {
			u.goff += d * 0.4
			moving = true
		}
	}
	if u.toast != "" && time.Now().Before(u.toastUntil) {
		moving = true
	}
	return moving
}
