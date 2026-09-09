package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func frontend(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/" || strings.HasPrefix(r.URL.Path, "/s/") {
		http.ServeFile(w, r, filepath.Join("dist", "index.html"))
		return
	}
	http.FileServer(http.Dir("dist")).ServeHTTP(w, r)
}

func main() {
	_ = os.MkdirAll(root, 0755)
	if err := load(); err != nil {
		fmt.Println("shares metadata unavailable:", err)
		shares = make(map[string]Share)
	}
	pruneExpired()
	go cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("/api/info", networkInfo)
	mux.HandleFunc("/api/shares", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			upload(w, r)
		case http.MethodDelete:
			revoke(w, r)
		case http.MethodGet:
			listShares(w, r)
		default:
			http.NotFound(w, r)
		}
	})
	mux.HandleFunc("/api/shares/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			revoke(w, r)
			return
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/qr"):
			qr(w, r)
		case strings.HasSuffix(r.URL.Path, "/zip"):
			downloadAll(w, r)
		case strings.Contains(r.URL.Path, "/files/"):
			download(w, r)
		default:
			getShare(w, r)
		}
	})
	mux.HandleFunc("/", frontend)

	server := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w, r)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		mux.ServeHTTP(w, r)
	})
	fmt.Println("Conecter listening on http://0.0.0.0:5173")
	if err := http.ListenAndServe(":5173", server); err != nil {
		fmt.Println("server stopped:", err)
	}
}
