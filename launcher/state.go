package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// State é o que o lançador lembra entre execuções (arquivo JSON no cartão).
type State struct {
	Favs       []string `json:"favoritos"` // chaves "sistema/arquivo"
	Recent     []string `json:"recentes"`  // mais recente primeiro
	Brightness int      `json:"brilho"`    // 0 a 100 (%), 0 = ainda não definido
	VolumeDB   int      `json:"volume_db"` // ganho dos jogos em dB, -40 a 0
	path       string
}

const maxRecent = 12

func LoadState(path string) *State {
	s := &State{VolumeDB: -8, path: path}
	if b, err := os.ReadFile(path); err == nil {
		json.Unmarshal(b, s)
	}
	s.path = path
	if s.VolumeDB > 0 || s.VolumeDB < -40 {
		s.VolumeDB = -8
	}
	return s
}

func (s *State) Save() {
	if s.path == "" {
		return
	}
	os.MkdirAll(filepath.Dir(s.path), 0o755)
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, s.path)
	}
}

func (s *State) IsFav(key string) bool {
	for _, k := range s.Favs {
		if k == key {
			return true
		}
	}
	return false
}

// ToggleFav marca ou desmarca e devolve o novo estado.
func (s *State) ToggleFav(key string) bool {
	for i, k := range s.Favs {
		if k == key {
			s.Favs = append(s.Favs[:i], s.Favs[i+1:]...)
			return false
		}
	}
	s.Favs = append(s.Favs, key)
	return true
}

func (s *State) AddRecent(key string) {
	out := []string{key}
	for _, k := range s.Recent {
		if k != key {
			out = append(out, k)
		}
	}
	if len(out) > maxRecent {
		out = out[:maxRecent]
	}
	s.Recent = out
}
