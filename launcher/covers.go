package main

import (
	"sync"
)

// coverBox é o lado da caixa onde a capa é encaixada (a coluna da esquerda da lista).
const (
	coverBox = 128
	coverMax = 8 // capas mantidas em cache (cada uma ocupa ~100 KB)
)

// coverCache carrega as capas dos jogos em segundo plano, uma de cada vez, e guarda poucas. Só a do jogo
// selecionado é pedida; se o usuário continuar rolando, o pedido mais novo substitui o anterior.
type coverCache struct {
	mu     sync.Mutex
	items  map[string]*Sprite // nil = falhou (não tenta de novo)
	order  []string           // do mais antigo ao mais novo, para descartar
	req    chan string
	notify chan struct{} // avisa o laço principal que uma capa ficou pronta
}

func newCoverCache() *coverCache {
	c := &coverCache{items: map[string]*Sprite{}, req: make(chan string, 1), notify: make(chan struct{}, 1)}
	go c.worker()
	return c
}

// Get devolve o sprite pronto da capa (nil se ainda não carregou ou se falhou) e se a capa já é conhecida.
func (c *coverCache) Get(path string) (*Sprite, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	sp, ok := c.items[path]
	return sp, ok
}

// Want pede a capa (se ainda não está no cache). O pedido mais novo substitui o que estiver esperando.
func (c *coverCache) Want(path string) {
	if _, ok := c.Get(path); ok {
		return
	}
	for {
		select {
		case c.req <- path:
			return
		default:
			select {
			case <-c.req: // descarta o pedido antigo
			default:
			}
		}
	}
}

func (c *coverCache) worker() {
	for path := range c.req {
		if _, ok := c.Get(path); ok {
			continue
		}
		sp := buildCover(path)
		c.mu.Lock()
		c.items[path] = sp
		c.order = append(c.order, path)
		for len(c.order) > coverMax {
			delete(c.items, c.order[0])
			c.order = c.order[1:]
		}
		c.mu.Unlock()
		select {
		case c.notify <- struct{}{}:
		default:
		}
	}
}

// buildCover lê a imagem, encaixa-a em coverBox x coverBox mantendo a proporção e devolve o sprite com cantos
// arredondados e sombra (nil se a imagem não puder ser lida).
func buildCover(path string) *Sprite {
	pic, err := loadImage(path)
	if err != nil {
		return nil
	}
	sw, sh := pic.Bounds().Dx(), pic.Bounds().Dy()
	scale := float64(coverBox) / float64(sw)
	if s2 := float64(coverBox) / float64(sh); s2 < scale {
		scale = s2
	}
	if scale > 1 {
		scale = 1 // capa pequena não é ampliada
	}
	dw, dh := int(float64(sw)*scale+0.5), int(float64(sh)*scale+0.5)
	if dw < 8 || dh < 8 {
		return nil
	}
	img := NewImg(dw, dh)
	img.Fill(0x10121F)
	img.DrawFit(pic, 0, 0, dw, dh)
	return newRoundSprite(img, 8, 14, 150)
}
