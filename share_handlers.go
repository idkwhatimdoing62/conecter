package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func upload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+1)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		jsonOut(w, map[string]string{"error": "上传失败：文件过大或请求无效"}, 400)
		return
	}
	defer r.MultipartForm.RemoveAll()
	parts := r.MultipartForm.File["files"]
	if len(parts) == 0 {
		jsonOut(w, map[string]string{"error": "没有收到文件"}, 400)
		return
	}
	if len(parts) > maxUploadFiles {
		jsonOut(w, map[string]string{"error": "一次最多上传 50 个文件"}, 400)
		return
	}
	id, err := code()
	if err != nil {
		jsonOut(w, map[string]string{"error": "无法生成分享码"}, 500)
		return
	}
	for {
		mu.RLock()
		_, ok := shares[id]
		mu.RUnlock()
		if !ok {
			break
		}
		id, err = code()
		if err != nil {
			jsonOut(w, map[string]string{"error": "无法生成分享码"}, 500)
			return
		}
	}
	now := time.Now()
	share := Share{ID: id, CreatedAt: now.UnixMilli(), Expires: now.Add(24 * time.Hour).UnixMilli()}
	created := []string{}
	failed := false
	for i, header := range parts {
		input, err := header.Open()
		if err != nil {
			failed = true
			break
		}
		name := safeFilename(header.Filename)
		path := filepath.Join(root, fmt.Sprintf("%s-%03d-%s", id, i, name))
		output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			_ = input.Close()
			failed = true
			break
		}
		size, copyErr := io.Copy(output, input)
		closeOutputErr := output.Close()
		closeInputErr := input.Close()
		if copyErr != nil || closeOutputErr != nil || closeInputErr != nil {
			_ = os.Remove(path)
			failed = true
			break
		}
		share.Files = append(share.Files, Asset{Name: name, Size: size, Path: path, Mime: header.Header.Get("Content-Type")})
		created = append(created, path)
	}
	if failed || len(share.Files) == 0 {
		for _, path := range created {
			_ = os.Remove(path)
		}
		jsonOut(w, map[string]string{"error": "没有收到有效文件"}, 400)
		return
	}
	mu.Lock()
	shares[id] = share
	mu.Unlock()
	if err := save(); err != nil {
		mu.Lock()
		delete(shares, id)
		mu.Unlock()
		for _, path := range created {
			_ = os.Remove(path)
		}
		jsonOut(w, map[string]string{"error": "保存分享信息失败"}, 500)
		return
	}
	jsonOut(w, map[string]any{"id": id, "expires": share.Expires, "url": "/s/" + id}, 200)
}

func getShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	if !allowLookup(r) {
		w.Header().Set("Retry-After", "60")
		jsonOut(w, map[string]string{"error": "请求过于频繁，请稍后再试"}, http.StatusTooManyRequests)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/shares/")
	if !validID(id) {
		jsonOut(w, map[string]string{"error": "分享不存在或已过期"}, 404)
		return
	}
	mu.RLock()
	share, ok := shares[id]
	mu.RUnlock()
	if !ok || share.Expires < time.Now().UnixMilli() {
		jsonOut(w, map[string]string{"error": "分享不存在或已过期"}, 404)
		return
	}
	type item struct {
		Name     string `json:"name"`
		Size     int64  `json:"size"`
		Download string `json:"download"`
	}
	files := []item{}
	for i, file := range share.Files {
		files = append(files, item{file.Name, file.Size, fmt.Sprintf("/api/shares/%s/files/%d", id, i)})
	}
	jsonOut(w, map[string]any{"id": id, "expires": share.Expires, "files": files}, 200)
}

func download(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/shares/"), "/files/")
	if len(parts) != 2 || !validID(parts[0]) {
		http.NotFound(w, r)
		return
	}
	mu.RLock()
	share, ok := shares[parts[0]]
	mu.RUnlock()
	if !ok || share.Expires < time.Now().UnixMilli() {
		http.NotFound(w, r)
		return
	}
	index, err := strconv.Atoi(parts[1])
	if err != nil || index < 0 || index >= len(share.Files) {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Disposition", attachmentDisposition(share.Files[index].Name))
	http.ServeFile(w, r, share.Files[index].Path)
}

func revoke(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/shares/")
	if !validID(id) {
		http.NotFound(w, r)
		return
	}
	mu.Lock()
	share, ok := shares[id]
	if ok {
		for _, file := range share.Files {
			_ = os.Remove(file.Path)
		}
		delete(shares, id)
	}
	mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	if err := save(); err != nil {
		jsonOut(w, map[string]string{"error": "保存分享状态失败"}, 500)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func listShares(w http.ResponseWriter, r *http.Request) {
	mu.RLock()
	defer mu.RUnlock()
	now := time.Now().UnixMilli()
	out := []publicShare{}
	for _, share := range shares {
		if share.Expires >= now {
			createdAt := share.CreatedAt
			if createdAt == 0 {
				createdAt = share.Expires - int64((24 * time.Hour).Milliseconds())
			}
			item := publicShare{ID: share.ID, CreatedAt: createdAt, Expires: share.Expires, Files: []publicFile{}}
			for _, file := range share.Files {
				item.Files = append(item.Files, publicFile{Name: file.Name, Size: file.Size})
			}
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt > out[j].CreatedAt })
	jsonOut(w, out, 200)
}
