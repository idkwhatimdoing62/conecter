package main

import (
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

func qr(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/shares/"), "/qr")
	if !validID(id) {
		http.NotFound(w, r)
		return
	}
	mu.RLock()
	share, ok := shares[id]
	mu.RUnlock()
	if !ok || share.Expires < time.Now().UnixMilli() {
		http.NotFound(w, r)
		return
	}
	target := r.URL.Query().Get("url")
	if target == "" {
		target = "http://" + r.Host + "/s/" + id
	}
	png, err := qrcode.Encode(target, qrcode.Medium, 256)
	if err != nil {
		http.Error(w, "qr encode failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

func downloadAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/shares/"), "/zip")
	if !validID(id) {
		http.NotFound(w, r)
		return
	}
	mu.RLock()
	share, ok := shares[id]
	mu.RUnlock()
	if !ok || share.Expires < time.Now().UnixMilli() {
		http.NotFound(w, r)
		return
	}
	type openedFile struct {
		file *os.File
		name string
	}
	opened := make([]openedFile, 0, len(share.Files))
	for _, file := range share.Files {
		input, err := os.Open(file.Path)
		if err != nil {
			for _, item := range opened {
				_ = item.file.Close()
			}
			http.Error(w, "分享文件暂时不可用", http.StatusNotFound)
			return
		}
		opened = append(opened, openedFile{file: input, name: file.Name})
	}
	defer func() {
		for _, item := range opened {
			_ = item.file.Close()
		}
	}()
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+id+`.zip"`)
	if r.Method == http.MethodHead {
		return
	}
	zipWriter := zip.NewWriter(w)
	for _, item := range opened {
		header := &zip.FileHeader{Name: safeFilename(item.name), Method: zip.Deflate}
		output, err := zipWriter.CreateHeader(header)
		if err != nil {
			log.Printf("create ZIP entry %q: %v", item.name, err)
			return
		}
		if _, err := io.Copy(output, item.file); err != nil {
			log.Printf("copy file %q into ZIP: %v", item.name, err)
			return
		}
	}
	if err := zipWriter.Close(); err != nil {
		log.Printf("close ZIP stream for %s: %v", id, err)
	}
}

func attachmentDisposition(name string) string {
	name = safeFilename(name)
	fallback := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(name)
	return fmt.Sprintf(`attachment; filename="%s"; filename*=UTF-8''%s`, fallback, url.PathEscape(name))
}
