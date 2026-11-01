package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	repeatDelay    = 350 * time.Millisecond
	repeatInterval = 100 * time.Millisecond
	frame          = 33 * time.Millisecond
	idleOff        = 90 * time.Second        // apaga a luz da tela depois deste tempo sem botões
	powerHold      = 1500 * time.Millisecond // segurar o botão de ligar por este tempo desliga
	batteryEvery   = 20 * time.Second
	version        = "0.2"
)

var debugInput = os.Getenv("X39_DEBUG") != ""

func isDir(k Key) bool { return k == KeyUp || k == KeyDown || k == KeyLeft || k == KeyRight }

// splash mostra "Carregando" com uma barra de progresso enquanto os cartões são pré-renderizados.
func splash(fb *Framebuffer, done, total int) {
	s := NewScreen(screenW, screenH)
	bg := pack565(0x0D, 0x10, 0x24)
	for i := range s.Pix {
		s.Pix[i] = bg
	}
	f := loadFont("bold22")
	s.TextC(f, screenW/2, 104, "Carregando…", colText, 255)
	s.FillRect(screenW/2-80, 148, 160, 6, 0xFFFFFF, 40)
	s.FillRect(screenW/2-80, 148, 160*done/max(total, 1), 6, 0x6EE7B7, 255)
	fb.Present(s)
}

// jumpLetter pula para o início da próxima (dir>0) ou da anterior (dir<0) letra da lista ordenada.
func jumpLetter(g []Game, cur, dir int) int {
	if len(g) == 0 {
		return 0
	}
	c := initial(g[cur].Name)
	if dir > 0 {
		for i := cur + 1; i < len(g); i++ {
			if initial(g[i].Name) != c {
				return i
			}
		}
		return cur
	}
	start := cur
	for start > 0 && initial(g[start-1].Name) == c {
		start--
	}
	if cur > start {
		return start
	}
	if start == 0 {
		return 0
	}
	p := start - 1
	pc := initial(g[p].Name)
	for p > 0 && initial(g[p-1].Name) == pc {
		p--
	}
	return p
}

// runDevice é o laço principal no console: desenha, lê os botões e abre os jogos.
// sigQuit é fechado quando o lançador recebe SIGTERM/SIGINT: o laço principal sai normalmente (devolve tela,
// terminal e luz) em vez de morrer no meio.
var sigQuit = make(chan struct{})

func runDevice(cfg Config, real []Sys, demo bool, autoMode string, runFor time.Duration) error {
	fb, err := OpenFramebuffer("/dev/fb0")
	if err != nil {
		return err
	}
	// guarda o conteúdo atual da tela para devolvê-lo ao sair (útil nos testes por ADB)
	orig := append([]byte(nil), fb.mem[:fb.pageSize*fb.pages]...)
	bl := FindBacklight()
	origBright := ""
	if bl != nil {
		origBright = bl.Raw()
	}
	defer func() {
		if bl != nil {
			bl.Power(true)
			bl.SetRaw(origBright) // devolve o brilho exato que estava
		}
		copy(fb.mem, orig)
		fb.Restore()
		fb.Close()
	}()
	in := StartInput("/dev/input/event1", "/dev/input/event0")
	StartHotplug(in, cfg.Card) // teclado e controles USB (porta USB-A 2), com plug a qualquer momento
	TakeTTY()
	defer ReleaseTTY()
	const devKeys, devPower = "/dev/input/event1", "/dev/input/event0"
	if cfg.Grab {
		GrabInput(devKeys, true)
		GrabInput(devPower, true)
	}
	switch autoMode {
	case "nav":
		go autoScript(in)
	case "launch":
		go autoLaunchScript(in, cfg.GameTime)
	}

	statePath := filepath.Join(cfg.Card, "data", "launcher.json")
	if demo {
		statePath = "" // o modo demonstração não grava nada
	}
	st := LoadState(statePath)

	// Recentes e Favoritos são "sistemas" virtuais no começo do carrossel
	index := map[string]Game{}
	for _, s := range real {
		for _, g := range s.Games {
			index[g.Key] = g
		}
	}
	virtual := func() (rec, fav []Game) {
		for _, k := range st.Recent {
			if g, ok := index[k]; ok {
				rec = append(rec, g)
			}
		}
		for _, k := range st.Favs {
			if g, ok := index[k]; ok {
				fav = append(fav, g)
			}
		}
		return
	}
	rec, fav := virtual()
	sys := append([]Sys{{System: RecentSystem, Games: rec}, {System: FavSystem, Games: fav}}, real...)
	n := len(sys)

	splash(fb, 0, n)
	t0 := time.Now()
	u := NewUI(sys, st, func(done, total int) { splash(fb, done, total) })
	fmt.Fprintf(os.Stderr, "interface construída em %v\n", time.Since(t0))
	u.target, u.pos, u.sel = 2, 2, 2 // começa no primeiro sistema real
	if n == 2 {
		u.target, u.pos, u.sel = 0, 0, 0
	}
	if bl != nil {
		if st.Brightness > 0 {
			bl.SetPercent(st.Brightness)
		} else {
			st.Brightness = bl.Percent()
		}
	}

	if len(real) == 0 {
		u.Toast("Nenhum sistema com jogos. Veja x39/sistemas e x39/roms no cartão", 8*time.Second, false)
	}
	var nDraw int
	var sumDraw, maxDraw time.Duration
	defer func() {
		if nDraw > 0 {
			fmt.Fprintf(os.Stderr, "desenho: %d quadros, média %.1f ms, máximo %.1f ms\n",
				nDraw, float64(sumDraw)/float64(nDraw)/1e6, float64(maxDraw)/1e6)
		}
	}()
	var quit <-chan time.Time
	if runFor > 0 {
		quit = time.After(runFor)
	}

	dirty := true
	var repKey Key
	var repAt time.Time
	var ignoreUntil time.Time // depois de um jogo: botões que ainda estavam na fila do kernel não contam
	lastInput := time.Now()
	sleeping := false
	var powerDown time.Time // zero = botão de ligar solto
	powerHandled := false
	nextBat := time.Now()
	nextUnblank := time.Now()
	var coverPath string  // capa do jogo selecionado na lista
	var coverAt time.Time // quando pedi-la (depois de 250 ms parado no mesmo jogo)
	coverAsked := true
	nextShow := time.Now() // reafirma a nossa página de vídeo (o firmware pode virar a página sem avisar)

	wake := func() {
		if sleeping {
			sleeping = false
			if bl != nil {
				bl.Power(true)
			}
			dirty = true
		}
		lastInput = time.Now()
	}

	// ---- Opções
	type optKind int
	const (
		oBright optKind = iota
		oVolume
		oInfo
		oReboot
		oShutdown
		oExitOriginal
		oClose
	)
	var optKinds []optKind
	rebuildOptions := func() {
		optKinds = optKinds[:0]
		var items []OptItem
		if bl != nil {
			optKinds = append(optKinds, oBright)
			items = append(items, OptItem{Label: "Brilho", Slider: true, Frac: float64(st.Brightness-5) / 95, Value: fmt.Sprintf("%d%%", st.Brightness)})
		}
		optKinds = append(optKinds, oVolume, oInfo, oReboot, oShutdown)
		items = append(items,
			OptItem{Label: "Volume dos jogos", Slider: true, Frac: float64(st.VolumeDB+40) / 40, Value: fmt.Sprintf("%d dB", st.VolumeDB)},
			OptItem{Label: "Informações"},
			OptItem{Label: "Reiniciar"},
			OptItem{Label: "Desligar"})
		if cfg.Hook {
			optKinds = append(optKinds, oExitOriginal)
			items = append(items, OptItem{Label: "Menu original do console"})
		}
		optKinds = append(optKinds, oClose)
		items = append(items, OptItem{Label: "Fechar"})
		u.opt.Items = items
		u.opt.Sel = clampInt(u.opt.Sel, 0, len(items)-1)
	}
	openOptions := func() {
		u.mode = modeOptions
		u.opt = OptionsView{}
		rebuildOptions()
	}
	// guarda de qual tela vieram as Opções
	prevMode := modeCarousel
	exitOriginal := false

	doShutdown := func(reboot bool) {
		if demo {
			u.Toast("Modo demonstração: não desliga", 2*time.Second, false)
			return
		}
		st.Save()
		u.Toast("Desligando…", 10*time.Second, false)
		if reboot {
			u.Toast("Reiniciando…", 10*time.Second, false)
		}
		u.Draw()
		fb.Present(u.scr)
		var err error
		if reboot {
			err = Reboot()
		} else {
			err = PowerOff()
		}
		if err != nil {
			u.Toast(fmt.Sprintf("Erro: %v", err), 4*time.Second, true)
		}
	}

	optPress := func(k Key) {
		v := &u.opt
		switch {
		case v.Confirm != "":
			if k == KeyA {
				c := v.Confirm
				v.Confirm = ""
				u.mode = prevMode
				if c == "Ir para o menu original?" {
					st.Save()
					exitOriginal = true
					return
				}
				doShutdown(c == "Reiniciar o console?")
			} else if k == KeyB {
				v.Confirm = ""
			}
			return
		case len(v.Lines) > 0:
			if k == KeyB || k == KeyA {
				v.Lines = nil
			}
			return
		}
		switch k {
		case KeyUp:
			v.Sel = clampInt(v.Sel-1, 0, len(v.Items)-1)
		case KeyDown:
			v.Sel = clampInt(v.Sel+1, 0, len(v.Items)-1)
		case KeyB, KeyMenu:
			u.mode = prevMode
			st.Save()
		case KeyLeft, KeyRight:
			d := 1
			if k == KeyLeft {
				d = -1
			}
			switch optKinds[v.Sel] {
			case oBright:
				st.Brightness = clampInt(st.Brightness+5*d, 5, 100)
				bl.SetPercent(st.Brightness)
			case oVolume:
				st.VolumeDB = clampInt(st.VolumeDB+2*d, -40, 0)
			}
			rebuildOptions()
		case KeyA:
			switch optKinds[v.Sel] {
			case oInfo:
				pct, ch := Battery()
				bat := "sem leitura"
				if pct >= 0 {
					bat = fmt.Sprintf("%d%%", pct)
					if ch {
						bat += " (carregando)"
					}
				}
				v.Lines = []string{
					"Lançador X39 versão " + version,
					"RAM livre: " + fmtMB(MemFreeMB()),
					"Cartão livre: " + fmtMB(DiskFreeMB(cfg.Card)),
					"Bateria: " + bat,
				}
			case oReboot:
				v.Confirm = "Reiniciar o console?"
			case oShutdown:
				v.Confirm = "Desligar o console?"
			case oExitOriginal:
				v.Confirm = "Ir para o menu original?"
			case oClose:
				u.mode = prevMode
				st.Save()
			}
		}
	}

	refreshVirtual := func() {
		r, f := virtual()
		u.SetVirtual(r, f)
	}

	launch := func(game Game) {
		if demo {
			u.Toast("Modo demonstração: não inicia jogos", 2*time.Second, false)
			return
		}
		msg := "Iniciando " + game.Name + "…"
		if len(FindPads()) > 0 && jailHas(cfg.Root, "input") {
			msg += "  (2 jogadores)"
		}
		u.Toast(msg, 30*time.Second, false)
		u.Draw()
		fb.Present(u.scr)
		st.AddRecent(game.Key)
		st.Save()
		if cfg.Grab {
			GrabInput(devKeys, false) // o RetroArch lê o teclado por js0 e não receberia nada com o teclado preso
		}
		lerr := cfg.Launch(game, st.VolumeDB)
		if cfg.Grab {
			GrabInput(devKeys, true)
		}
		fmt.Fprintf(os.Stderr, "jogo terminou: erro=%v\n", lerr)
		u.ClearToast()
		if lerr != nil {
			u.Toast(fmt.Sprintf("Erro: %v", lerr), 5*time.Second, true)
		}
		// o RetroArch pode ter mudado o modo de vídeo: reabre o framebuffer
		fb.Close()
		for i := 0; i < 20; i++ {
			if fb, err = OpenFramebuffer("/dev/fb0"); err == nil {
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		// descarta botões acumulados durante o jogo. O kernel ainda pode entregar botões atrasados
		// (por exemplo o "A" que confirmou "fechar conteúdo"): espera um instante, descarta de novo
		// e ignora botões por mais um tempo, senão eles contam como botões novos (reabria o jogo).
		for round := 0; round < 2; round++ {
			time.Sleep(250 * time.Millisecond)
		drain:
			for {
				select {
				case <-in:
				default:
					break drain
				}
			}
		}
		ignoreUntil = time.Now().Add(400 * time.Millisecond)
		repKey = KeyNone
		TakeTTY() // o RetroArch devolveu o terminal ao modo texto
		if fb != nil {
			fb.Wake() // o vídeo pode ter ficado bloqueado depois do RetroArch (imagem na memória, tela preta)
		}
		wake() // se a tela estava apagada, acende
		if bl != nil {
			// o RetroArch/SDL pode ter apagado a luz ao sair (bl_power=1) sem o lançador saber: sempre religa
			bl.Power(true)
			bl.SetPercent(st.Brightness)
		}
		dirty = true // o RetroArch deixou a tela como quis (preta): sempre redesenha ao voltar
		refreshVirtual()
		fmt.Fprintf(os.Stderr, "recentes agora: %d, favoritos: %d\n", len(u.sys[0].Games), len(u.sys[1].Games))
		lastInput = time.Now()
	}

	press := func(k Key) {
		dirty = true
		switch u.mode {
		case modeOptions:
			optPress(k)
			return
		case modeCarousel:
			switch k {
			case KeyLeft:
				u.target--
			case KeyRight:
				u.target++
			case KeyA:
				u.mode, u.gsel, u.goff = modeList, 0, 0
			case KeyMenu:
				prevMode = modeCarousel
				openOptions()
			}
			u.sel = mod(u.target, n)
			return
		}
		g := u.sys[u.sel].Games
		switch k {
		case KeyUp:
			u.gsel = clampInt(u.gsel-1, 0, max(len(g)-1, 0))
		case KeyDown:
			u.gsel = clampInt(u.gsel+1, 0, max(len(g)-1, 0))
		case KeyL1:
			u.gsel = clampInt(u.gsel-listRows, 0, max(len(g)-1, 0))
		case KeyR1:
			u.gsel = clampInt(u.gsel+listRows, 0, max(len(g)-1, 0))
		case KeyL2, KeyR2:
			if u.sel >= 2 && len(g) > 0 { // pular por letra só faz sentido nas listas ordenadas
				d := 1
				if k == KeyL2 {
					d = -1
				}
				u.gsel = jumpLetter(g, u.gsel, d)
			}
		case KeyB:
			u.mode = modeCarousel
		case KeyMenu:
			prevMode = modeList
			openOptions()
		case KeyY:
			if len(g) == 0 {
				return
			}
			game := g[u.gsel]
			if st.ToggleFav(game.Key) {
				u.Toast("★ Adicionado aos favoritos", 1200*time.Millisecond, false)
			} else {
				u.Toast("Removido dos favoritos", 1200*time.Millisecond, false)
			}
			st.Save()
			refreshVirtual()
			u.gsel = clampInt(u.gsel, 0, max(len(u.sys[u.sel].Games)-1, 0))
		case KeyA:
			if len(g) > 0 {
				launch(g[u.gsel])
			}
		}
	}

	handle := func(ev Event) {
		if time.Now().Before(ignoreUntil) {
			return
		}
		fb.Unblank() // um botão sempre garante o vídeo ligado
		if debugInput {
			fmt.Fprintf(os.Stderr, "ev tecla=%d desce=%v rep=%d dormindo=%v modo=%d\n", ev.Key, ev.Down, repKey, sleeping, u.mode)
		}
		if !ev.Down && ev.Key == repKey {
			repKey = KeyNone
		}
		if sleeping {
			if ev.Down {
				wake()
			}
			return // o botão que acordou a tela não faz mais nada
		}
		lastInput = time.Now()
		if ev.Key == KeyPower {
			if ev.Down {
				powerDown, powerHandled = time.Now(), false
			} else {
				if !powerDown.IsZero() && !powerHandled && bl != nil {
					// toque curto: apaga a tela
					sleeping = true
					repKey = KeyNone
					bl.Power(false)
				}
				powerDown = time.Time{}
			}
			return
		}
		if ev.Down {
			if isDir(ev.Key) {
				repKey, repAt = ev.Key, time.Now().Add(repeatDelay)
			}
			press(ev.Key)
		} else if ev.Key == repKey {
			repKey = KeyNone
		}
	}

	for {
		// 1) trata TODOS os botões pendentes antes de qualquer repetição: um quadro lento
		//    nunca pode fazer a repetição disparar antes de o botão solto ser lido
		for pending := true; pending; {
			select {
			case ev := <-in:
				handle(ev)
			default:
				pending = false
			}
		}
		now := time.Now()
		// 2) repetição de direcional enquanto o botão está segurado
		if repKey != KeyNone && !now.Before(repAt) && !sleeping {
			press(repKey)
			repAt = now.Add(repeatInterval)
		}
		// 3) botão de ligar segurado: desliga
		if !powerDown.IsZero() && !powerHandled && now.Sub(powerDown) >= powerHold {
			powerHandled = true
			doShutdown(false)
		}
		// 4) inatividade: apaga a luz da tela
		if !sleeping && bl != nil && now.Sub(lastInput) >= idleOff && u.mode != modeOptions {
			sleeping = true
			repKey = KeyNone
			bl.Power(false)
		}
		// 4b) capa do jogo selecionado: só pede depois de 250 ms parado nele (rolar a lista não carrega capas)
		{
			cur := ""
			if u.mode == modeList && u.sel >= 0 && u.sel < len(u.sys) {
				if g := u.sys[u.sel].Games; u.gsel >= 0 && u.gsel < len(g) {
					cur = g[u.gsel].Cover
				}
			}
			if cur != coverPath {
				coverPath, coverAt, coverAsked = cur, now.Add(250*time.Millisecond), cur == ""
			}
			if !coverAsked && !now.Before(coverAt) {
				coverAsked = true
				u.covers.Want(coverPath)
			}
		}
		// 5) reafirma a página de vídeo própria
		if !sleeping && !now.Before(nextShow) {
			nextShow = now.Add(250 * time.Millisecond)
			fb.Show()
			if now.After(nextUnblank) {
				nextUnblank = now.Add(2 * time.Second)
				fb.Unblank()
			}
		}
		// 5b) bateria
		if !now.Before(nextBat) {
			nextBat = now.Add(batteryEvery)
			if p, c := Battery(); p != u.battery || c != u.charging {
				u.battery, u.charging = p, c
				dirty = true
			}
		}
		// 6) anima e desenha
		moving := false
		if !sleeping {
			moving = u.Step()
			if dirty || moving {
				t := time.Now()
				u.Draw()
				fb.Present(u.scr)
				dt := time.Since(t)
				if nDraw == 0 {
					if cfg.OKFile != "" {
						os.Remove(cfg.OKFile) // chegou à primeira tela: o boot deu certo
					}
					if cfg.Vivo != "" {
						os.WriteFile(cfg.Vivo, nil, 0o644) // avisa o temporizador de segurança
					}
				}
				nDraw++
				sumDraw += dt
				if dt > maxDraw {
					maxDraw = dt
				}
				dirty = false
			}
		}
		if exitOriginal {
			return nil
		}
		// 7) dorme até o próximo evento que importa
		next := now.Add(time.Hour)
		earlier := func(t time.Time) {
			if t.Before(next) {
				next = t
			}
		}
		if moving {
			earlier(now.Add(frame))
		}
		if repKey != KeyNone && !sleeping {
			earlier(repAt)
		}
		if !powerDown.IsZero() && !powerHandled {
			earlier(powerDown.Add(powerHold))
		}
		if !sleeping && bl != nil && u.mode != modeOptions {
			earlier(lastInput.Add(idleOff))
		}
		earlier(nextBat)
		if !coverAsked {
			earlier(coverAt)
		}
		if !sleeping {
			earlier(nextShow)
		}
		select {
		case ev := <-in:
			handle(ev)
		case <-u.covers.notify:
			dirty = true // uma capa ficou pronta
		case <-quit:
			return nil
		case <-sigQuit:
			return nil
		case <-time.After(time.Until(next)):
		}
	}
}

// autoScript injeta toques de teste no lugar de botões reais (só com -auto).
func autoScript(in chan<- Event) {
	tap := func(k Key, gap time.Duration) {
		in <- Event{k, true}
		time.Sleep(40 * time.Millisecond)
		in <- Event{k, false}
		time.Sleep(gap)
	}
	time.Sleep(3 * time.Second)
	for i := 0; i < 5; i++ {
		tap(KeyRight, 600*time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		tap(KeyLeft, 600*time.Millisecond)
	}
	in <- Event{KeyRight, true} // segura à direita por 0,75 s: 1 passo + repetições
	time.Sleep(750 * time.Millisecond)
	in <- Event{KeyRight, false}
	time.Sleep(1200 * time.Millisecond)
	tap(KeyA, 1200*time.Millisecond) // abre a lista
	for i := 0; i < 10; i++ {
		tap(KeyDown, 250*time.Millisecond)
	}
	tap(KeyY, 1500*time.Millisecond) // favorita o jogo atual
	tap(KeyR2, 700*time.Millisecond) // pula de letra
	tap(KeyL2, 700*time.Millisecond)
	tap(KeyR1, 600*time.Millisecond)
	tap(KeyL1, 600*time.Millisecond)
	tap(KeyMenu, 1200*time.Millisecond) // Opções
	tap(KeyRight, 500*time.Millisecond) // brilho +5
	tap(KeyLeft, 500*time.Millisecond)
	tap(KeyDown, 500*time.Millisecond) // volume
	tap(KeyRight, 500*time.Millisecond)
	tap(KeyDown, 500*time.Millisecond) // informações
	tap(KeyA, 1500*time.Millisecond)
	tap(KeyB, 600*time.Millisecond)
	tap(KeyB, 800*time.Millisecond)  // fecha as Opções
	tap(KeyB, 1000*time.Millisecond) // volta ao carrossel
	for i := 0; i < 4; i++ {
		tap(KeyLeft, 400*time.Millisecond)
	}
}

// autoLaunchScript abre o primeiro jogo do primeiro sistema, o deixa rodar e confere o retorno (só com -autolaunch).
func autoLaunchScript(in chan<- Event, gameSecs int) {
	tap := func(k Key, gap time.Duration) {
		in <- Event{k, true}
		time.Sleep(40 * time.Millisecond)
		in <- Event{k, false}
		time.Sleep(gap)
	}
	time.Sleep(5 * time.Second)
	tap(KeyA, 1500*time.Millisecond) // abre a lista do sistema
	tap(KeyA, 1000*time.Millisecond) // inicia o jogo
	time.Sleep(time.Duration(gameSecs+14) * time.Second)
	tap(KeyB, 1000*time.Millisecond) // volta ao carrossel
	tap(KeyLeft, 700*time.Millisecond)
	tap(KeyLeft, 700*time.Millisecond) // chega em Recentes
	tap(KeyA, 3000*time.Millisecond)   // abre a lista de Recentes
}
