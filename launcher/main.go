package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

func savePNG(c *Screen, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, c.W, c.H))
	for i, p := range c.Pix {
		r, g, b := uint32(p>>11)&31, uint32(p>>5)&63, uint32(p)&31
		img.Pix[i*4+0] = uint8(r<<3 | r>>2)
		img.Pix[i*4+1] = uint8(g<<2 | g>>4)
		img.Pix[i*4+2] = uint8(b<<3 | b>>2)
		img.Pix[i*4+3] = 255
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func fakeSystems(list []System) []Sys {
	nomes := []string{"Alex Kidd in Miracle World", "Sonic the Hedgehog", "Wonder Boy III - The Dragon's Trap",
		"Phantasy Star", "Golden Axe Warrior", "Psycho Fox", "Rastan", "Shinobi", "Fantasy Zone", "R-Type",
		"Space Harrier", "Double Dragon", "Out Run", "Choplifter", "Castle of Illusion Starring Mickey Mouse"}
	var out []Sys
	for k, s := range list {
		n := 80 - k*5
		var gs []Game
		for i := 0; i < n; i++ {
			nm := nomes[(i+k)%len(nomes)]
			gs = append(gs, Game{Name: nm, File: "x", SysID: s.ID, Key: fmt.Sprintf("%s/%s-%d", s.ID, nm, i), Size: int64(32768 * (1 + i%12))})
		}
		out = append(out, Sys{System: s, Games: gs})
	}
	return out
}

func main() {
	preview := flag.String("preview", "", "grava imagens de prévia da interface nesta pasta e sai")
	demo := flag.Bool("demo", false, "usa sistemas e jogos de mentira (não inicia nada)")
	root := flag.String("root", "/tmp/x39", "pasta em RAM com app/, rt/lib, x39jail e tinymix")
	auto := flag.Bool("auto", false, "injeta toques de teste (para medir o desempenho sem ninguém na frente)")
	autoLaunch := flag.Bool("autolaunch", false, "abre o primeiro jogo sozinho e confere o retorno (teste)")
	hook := flag.Bool("watchdog", false, "modo de gancho de boot: alimenta o watchdog e oferece voltar ao menu original")
	vivo := flag.String("vivo", "", "arquivo criado depois da primeira tela desenhada (desarma o temporizador de segurança do gancho)")
	okFile := flag.String("okfile", "", "arquivo removido depois da primeira tela desenhada (proteção contra reinício em laço)")
	gameTime := flag.Int("gametime", 0, "encerra cada jogo depois de N segundos (para testes)")
	runFor := flag.Int("t", 0, "encerra sozinho depois de N segundos (para testes)")
	grab := flag.Bool("grab", false, "toma o uso exclusivo dos botões (o menu original/msg_server deixa de reagir ao botão de ligar)")
	card := flag.String("card", "/mnt/card/x39", "pasta do nosso cartão (cores/, roms/, data/)")
	flag.Parse()
	go func() { // SIGTERM/SIGINT: encerra o jogo em andamento (se houver) e sai normalmente
		c := make(chan os.Signal, 1)
		signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)
		<-c
		close(sigQuit)
		StopGame()
	}()
	if *preview != "" {
		if err := os.MkdirAll(*preview, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		t0 := time.Now()
		var real []Sys
		cardGiven := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "card" {
				cardGiven = true
			}
		})
		if cardGiven { // prévia com os sistemas e jogos de um cartão (real ou simulado)
			var warn []string
			systemsLoaded, warn = LoadSystems(filepath.Join(*card, "sistemas"))
			for _, w := range warn {
				fmt.Fprintln(os.Stderr, "aviso:", w)
			}
			real = Scan(filepath.Join(*card, "roms"), systemsLoaded)
		} else {
			systemsLoaded = demoSystems()
			real = fakeSystems(systemsLoaded)
		}
		if len(real) < 3 {
			fmt.Fprintln(os.Stderr, "a prévia precisa de pelo menos 3 sistemas com jogos")
			os.Exit(1)
		}
		st := &State{VolumeDB: -8, Brightness: 70}
		rec := append([]Game(nil), real[0].Games[:min(5, len(real[0].Games))]...)
		fav := append([]Game(nil), real[2].Games[:min(3, len(real[2].Games))]...)
		for _, g := range fav {
			st.Favs = append(st.Favs, g.Key)
		}
		sys := append([]Sys{{System: RecentSystem, Games: rec}, {System: FavSystem, Games: fav}}, real...)
		u := NewUI(sys, st, nil)
		u.battery, u.charging = 73, false
		fmt.Printf("construção da interface: %v\n", time.Since(t0))
		u.sel, u.pos, u.target = 2, 2, 2
		u.Draw()
		savePNG(u.scr, filepath.Join(*preview, "01-carrossel.png"))
		u.pos = 3.45
		u.Draw()
		savePNG(u.scr, filepath.Join(*preview, "02-carrossel-animando.png"))
		u.sel, u.pos, u.target, u.mode, u.gsel, u.goff = 4, 4, 4, 1, 3, 1.3
		if g := u.sys[u.sel].Games; u.gsel < len(g) && g[u.gsel].Cover != "" { // espera a capa carregar
			u.covers.Want(g[u.gsel].Cover)
			select {
			case <-u.covers.notify:
			case <-time.After(3 * time.Second):
			}
		}
		u.Draw()
		savePNG(u.scr, filepath.Join(*preview, "03-lista.png"))
		u.sel, u.pos, u.target, u.mode, u.gsel, u.goff = 1, 1, 1, 1, 1, 0
		u.Draw()
		savePNG(u.scr, filepath.Join(*preview, "04-favoritos.png"))
		u.sel, u.pos, u.target, u.mode = 2, 2, 2, 2
		u.opt = OptionsView{Sel: 1, Items: []OptItem{
			{Label: "Brilho", Slider: true, Frac: 0.68, Value: "70%"},
			{Label: "Volume dos jogos", Slider: true, Frac: 0.8, Value: "-8 dB"},
			{Label: "Informações"}, {Label: "Reiniciar"}, {Label: "Desligar"}, {Label: "Fechar"}}}
		u.Draw()
		savePNG(u.scr, filepath.Join(*preview, "05-opcoes.png"))
		// medição do custo de um quadro no PC (referência; no console será bem maior)
		t0 = time.Now()
		for i := 0; i < 200; i++ {
			u.mode, u.pos = 0, float64(i%11)+0.4
			u.Draw()
		}
		fmt.Printf("quadro do carrossel (PC): %.2f ms\n", float64(time.Since(t0))/200/1e6)
		fmt.Println("prévias em", *preview)
		return
	}
	var sys []Sys
	if *demo {
		systemsLoaded = demoSystems()
		sys = fakeSystems(systemsLoaded)
	} else {
		var warn []string
		systemsLoaded, warn = LoadSystems(filepath.Join(*card, "sistemas"))
		for _, w := range warn {
			fmt.Fprintln(os.Stderr, "aviso:", w)
		}
		sys = Scan(filepath.Join(*card, "roms"), systemsLoaded)
	}
	mode := ""
	if *auto {
		mode = "nav"
	}
	if *autoLaunch {
		mode = "launch"
	}
	var wd *Watchdog
	if *hook {
		wd = StartWatchdog(30)
	}
	err := runDevice(Config{Root: *root, Card: *card, GameTime: *gameTime, Hook: *hook, OKFile: *okFile, Vivo: *vivo, Grab: *grab}, sys, *demo, mode, time.Duration(*runFor)*time.Second)
	wd.Stop() // desarma limpo antes de o menu original assumir
	if err != nil {
		fmt.Fprintln(os.Stderr, "erro:", err)
		os.Exit(1)
	}
}
