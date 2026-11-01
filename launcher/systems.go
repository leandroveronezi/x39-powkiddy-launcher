package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

// System descreve um sistema que o lançador sabe abrir. Todos vêm do cartão:
//
//	x39/sistemas/<id>/sistema.json   (+ console.png|jpg|jpeg opcional)
//	x39/cores/<core>.so
//	x39/roms/<id>/...
type System struct {
	ID    string
	Name  string
	Short string
	Core  string
	Exts  []string
	C1    uint32 // cor de cima do degradê do cartão
	C2    uint32 // cor de baixo
	Image string // caminho da imagem do console ("" = só o degradê)
	Opts  map[string]string
	Order int
}

// sistemaJSON é o formato do arquivo sistema.json.
type sistemaJSON struct {
	Nome      string         `json:"nome"`
	Sigla     string         `json:"sigla"`
	Core      string         `json:"core"`
	Extensoes []string       `json:"extensoes"`
	Cor1      string         `json:"cor1"`
	Cor2      string         `json:"cor2"`
	Imagem    string         `json:"imagem"`
	Ordem     *int           `json:"ordem"`
	RetroArch map[string]any `json:"retroarch"`
}

var (
	idRe     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	optKeyRe = regexp.MustCompile(`^[a-z0-9_]+$`)
	// opções do RetroArch que o JSON NÃO pode mexer (caminhos, drivers, rede, comandos)
	optDeny = []string{"directory", "path", "dir", "file", "url", "cmd", "netplay", "network", "driver", "login", "password", "log"}
)

// systemsLoaded é o registro dos sistemas carregados (consultado por systemByID).
var systemsLoaded []System

func systemByID(id string) (System, bool) {
	for _, s := range systemsLoaded {
		if s.ID == id {
			return s, true
		}
	}
	return System{}, false
}

// Sistemas virtuais, sempre no início do carrossel (mesmo vazios, para o usuário descobrir o recurso).
var (
	RecentSystem = System{ID: "recentes", Name: "Recentes", Short: "▶", C1: 0x2DD4BF, C2: 0x0F5F58}
	FavSystem    = System{ID: "favoritos", Name: "Favoritos", Short: "★", C1: 0xFBBF24, C2: 0x8A3A0B}
)

func parseColor(s string, def uint32) uint32 {
	s = strings.TrimPrefix(strings.TrimSpace(s), "#")
	if len(s) != 6 {
		return def
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return def
	}
	return uint32(v)
}

// validOpts filtra as opções do RetroArch do JSON: chaves simples e valores sem aspas nem quebras de linha;
// caminhos, drivers, rede e comandos ficam de fora (quem manda neles é o lançador).
func validOpts(in map[string]any) (map[string]string, []string) {
	out := map[string]string{}
	var warn []string
	for k, v := range in {
		var val string
		switch x := v.(type) {
		case string:
			val = x
		case bool:
			val = strconv.FormatBool(x)
		case float64:
			val = strconv.FormatFloat(x, 'f', -1, 64)
		default:
			warn = append(warn, fmt.Sprintf("opção %q ignorada (valor não é texto, número ou verdadeiro/falso)", k))
			continue
		}
		bad := !optKeyRe.MatchString(k) || strings.ContainsAny(val, "\"\r\n") || len(val) > 200
		for _, d := range optDeny {
			if strings.Contains(k, d) {
				bad = true
			}
		}
		if bad {
			warn = append(warn, fmt.Sprintf("opção %q não permitida", k))
			continue
		}
		out[k] = val
	}
	return out, warn
}

// LoadSystems lê x39/sistemas/*/sistema.json. Um sistema com erro é ignorado (e avisado), nunca derruba o lançador.
func LoadSystems(dir string) ([]System, []string) {
	var out []System
	var warn []string
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil, []string{fmt.Sprintf("pasta de sistemas não encontrada: %s", dir)}
	}
	for _, e := range ents {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		id := e.Name()
		sd := filepath.Join(dir, id)
		w := func(f string, a ...any) { warn = append(warn, id+": "+fmt.Sprintf(f, a...)) }
		if !idRe.MatchString(id) {
			w("nome de pasta inválido (use letras, números, ponto, hífen ou sublinhado)")
			continue
		}
		raw, err := os.ReadFile(filepath.Join(sd, "sistema.json"))
		if err != nil {
			w("sem sistema.json")
			continue
		}
		var j sistemaJSON
		if err := json.Unmarshal(raw, &j); err != nil {
			w("sistema.json inválido: %v", err)
			continue
		}
		if j.Core == "" || filepath.Base(j.Core) != j.Core || !strings.HasSuffix(j.Core, ".so") {
			w("\"core\" deve ser o nome de um arquivo .so em x39/cores (sem pastas)")
			continue
		}
		var exts []string
		for _, x := range j.Extensoes {
			x = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(x), "."))
			if x != "" {
				exts = append(exts, x)
			}
		}
		if len(exts) == 0 {
			w("\"extensoes\" está vazio")
			continue
		}
		s := System{ID: id, Name: j.Nome, Short: j.Sigla, Core: j.Core, Exts: exts, Order: 100,
			C1: parseColor(j.Cor1, 0x6B8AAE), C2: parseColor(j.Cor2, 0x2A3C57)}
		if s.Name == "" {
			s.Name = id
		}
		if s.Short == "" {
			s.Short = strings.ToUpper(s.Name)
		}
		if utf8.RuneCountInString(s.Short) > 5 {
			s.Short = string([]rune(s.Short)[:5])
		}
		if j.Ordem != nil {
			s.Order = *j.Ordem
		}
		cands := []string{"console.png", "console.jpg", "console.jpeg"}
		if j.Imagem != "" {
			if filepath.Base(j.Imagem) != j.Imagem {
				w("\"imagem\" deve ser um arquivo da própria pasta do sistema")
			} else {
				cands = []string{j.Imagem}
			}
		}
		for _, c := range cands {
			if st, err := os.Stat(filepath.Join(sd, c)); err == nil && !st.IsDir() {
				s.Image = filepath.Join(sd, c)
				break
			}
		}
		var ow []string
		s.Opts, ow = validOpts(j.RetroArch)
		for _, m := range ow {
			w("%s", m)
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Order != out[b].Order {
			return out[a].Order < out[b].Order
		}
		return out[a].Name < out[b].Name
	})
	return out, warn
}
