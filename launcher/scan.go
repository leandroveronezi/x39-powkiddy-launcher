package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"
)

var tagRe = regexp.MustCompile(`\s*[\(\[][^\)\]]*[\)\]]`)

// cleanName tira etiquetas como (USA), (Rev 1) e [!] do nome mostrado.
func cleanName(n string) string {
	c := strings.TrimSpace(tagRe.ReplaceAllString(n, ""))
	if c == "" {
		return n
	}
	return c
}

// initial devolve a letra de agrupamento do nome (A-Z) ou '#' para números e símbolos.
func initial(n string) rune {
	for _, r := range n {
		r = unicode.ToUpper(r)
		if r >= 'A' && r <= 'Z' {
			return r
		}
		return '#'
	}
	return '#'
}

// isImage diz se o arquivo é uma imagem que o lançador sabe ler como capa.
func isImage(name string) bool { return hasExt(name, []string{"jpg", "jpeg", "png"}) }

// pickCover escolhe a capa de um jogo entre as imagens da mesma pasta: a de mesmo nome do ROM; na pasta do
// próprio jogo (own=true), na falta dela, a primeira imagem (em ordem alfabética).
func pickCover(rom string, images []string, own bool) string {
	stem := strings.ToLower(strings.TrimSuffix(filepath.Base(rom), filepath.Ext(rom)))
	for _, im := range images {
		if strings.ToLower(strings.TrimSuffix(filepath.Base(im), filepath.Ext(im))) == stem {
			return im
		}
	}
	if own && len(images) > 0 {
		return images[0]
	}
	return ""
}

func hasExt(name string, exts []string) bool {
	e := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	for _, x := range exts {
		if e == x {
			return true
		}
	}
	return false
}

func prettyName(file string) string {
	n := strings.TrimSuffix(file, filepath.Ext(file))
	n = strings.ReplaceAll(n, "_", " ")
	return strings.TrimSpace(n)
}

// Scan procura, em romRoot/<id>/, os jogos de cada sistema (arquivos diretos ou em subpastas de um nível).
// Só devolve os sistemas que têm pelo menos um jogo.
func Scan(romRoot string, systems []System) []Sys {
	var out []Sys
	for _, s := range systems {
		dir := filepath.Join(romRoot, s.ID)
		var games []Game
		ents, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		add := func(path, name string, size int64) {
			if hasExt(name, s.Exts) {
				rel, _ := filepath.Rel(dir, path)
				games = append(games, Game{Name: prettyName(name), File: path, Size: size, SysID: s.ID, Key: s.ID + "/" + rel})
			}
		}
		var flatImages []string // imagens soltas ao lado dos ROMs da pasta do sistema
		for _, e := range ents {
			if !e.IsDir() && e.Name() != "" && e.Name()[0] != '.' && isImage(e.Name()) {
				flatImages = append(flatImages, filepath.Join(dir, e.Name()))
			}
		}
		for _, e := range ents {
			if e.Name() == "" || e.Name()[0] == '.' {
				continue
			}
			p := filepath.Join(dir, e.Name())
			if e.IsDir() {
				sub, err := os.ReadDir(p)
				if err != nil {
					continue
				}
				start := len(games)
				var images []string
				for _, se := range sub {
					if !se.IsDir() && se.Name()[0] != '.' {
						if isImage(se.Name()) {
							images = append(images, filepath.Join(p, se.Name()))
						} else if info, err := se.Info(); err == nil {
							add(filepath.Join(p, se.Name()), se.Name(), info.Size())
						}
					}
				}
				for i := start; i < len(games); i++ {
					games[i].Cover = pickCover(games[i].File, images, true)
				}
			} else if info, err := e.Info(); err == nil {
				n := len(games)
				add(p, e.Name(), info.Size())
				if len(games) > n {
					games[n].Cover = pickCover(p, flatImages, false)
				}
			}
		}
		if len(games) == 0 {
			continue
		}
		// nomes limpos, exceto quando dois jogos ficariam com o mesmo nome (aí mantém o original)
		seen := map[string]int{}
		for _, g := range games {
			seen[strings.ToLower(cleanName(g.Name))]++
		}
		for i := range games {
			if c := cleanName(games[i].Name); seen[strings.ToLower(c)] == 1 {
				games[i].Name = c
			}
		}
		sort.Slice(games, func(i, j int) bool {
			return strings.ToLower(games[i].Name) < strings.ToLower(games[j].Name)
		})
		out = append(out, Sys{System: s, Games: games})
	}
	return out
}
