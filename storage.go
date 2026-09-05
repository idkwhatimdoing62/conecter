package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const sharesFile = "shares.json"

func save() error {
	mu.RLock()
	b, err := json.Marshal(shares)
	mu.RUnlock()
	if err != nil {
		return fmt.Errorf("marshal shares: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(sharesFile), ".shares-*.tmp")
	if err != nil {
		return fmt.Errorf("create shares temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0644); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod shares temp file: %w", err)
	}
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write shares temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync shares temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close shares temp file: %w", err)
	}
	if err := os.Rename(tmpName, sharesFile); err != nil {
		// Windows cannot rename over an existing file. Keep the safe temp write,
		// then use a replacement fallback for that platform.
		if removeErr := os.Remove(sharesFile); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("replace shares file: %w", err)
		}
		if renameErr := os.Rename(tmpName, sharesFile); renameErr != nil {
			return fmt.Errorf("rename shares file: %w", renameErr)
		}
	}
	return nil
}

func load() error {
	b, err := os.ReadFile(sharesFile)
	if errors.Is(err, os.ErrNotExist) {
		shares = make(map[string]Share)
		return nil
	}
	if err != nil {
		return fmt.Errorf("read shares file: %w", err)
	}
	loaded := make(map[string]Share)
	if err := json.Unmarshal(b, &loaded); err != nil {
		return fmt.Errorf("parse shares file: %w", err)
	}
	if loaded == nil {
		loaded = make(map[string]Share)
	}
	shares = loaded
	return nil
}

func pruneExpired() {
	now := time.Now().UnixMilli()
	var expired []Share
	mu.Lock()
	for id, share := range shares {
		if share.Expires < now {
			expired = append(expired, share)
			delete(shares, id)
		}
	}
	mu.Unlock()
	if len(expired) == 0 {
		return
	}
	for _, share := range expired {
		for _, file := range share.Files {
			if err := os.Remove(file.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
				log.Printf("remove expired file %q: %v", file.Path, err)
			}
		}
	}
	if err := save(); err != nil {
		log.Printf("persist expired-share cleanup: %v", err)
	}
}

func cleanup() {
	pruneExpired()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		pruneExpired()
	}
}
