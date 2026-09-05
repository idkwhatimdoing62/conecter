package main

import (
	"crypto/rand"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
)

type Asset struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	Path string `json:"path"`
	Mime string `json:"mime"`
}

type Share struct {
	ID        string  `json:"id"`
	CreatedAt int64   `json:"createdAt"`
	Expires   int64   `json:"expires"`
	Files     []Asset `json:"files"`
}

type publicFile struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type publicShare struct {
	ID        string       `json:"id"`
	CreatedAt int64        `json:"createdAt"`
	Expires   int64        `json:"expires"`
	Files     []publicFile `json:"files"`
}

var mu sync.RWMutex
var shares = map[string]Share{}
var root = "uploads"

const maxUploadBytes int64 = 5 << 30
const maxUploadFiles = 50

func validID(id string) bool {
	if len(id) != 6 {
		return false
	}
	for _, c := range id {
		if !(c >= 'A' && c <= 'Z' || c >= '2' && c <= '9') {
			return false
		}
	}
	return true
}

func safeFilename(raw string) string {
	name := filepath.Base(strings.TrimSpace(raw))
	if name == "." || name == "" {
		return "unnamed"
	}
	var b strings.Builder
	for _, r := range name {
		if r < 32 || r == 127 {
			b.WriteByte('_')
		} else {
			b.WriteRune(r)
		}
	}
	name = strings.TrimSpace(b.String())
	if name == "" {
		name = "unnamed"
	}
	runes := []rune(name)
	if len(runes) > 180 {
		name = string(runes[:180])
	}
	return name
}

func code() (string, error) {
	const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b), nil
}

func jsonOut(w http.ResponseWriter, v any, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
