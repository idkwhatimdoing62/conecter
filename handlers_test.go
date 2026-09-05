package main

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestValidID(t *testing.T) {
	for _, test := range []struct {
		id    string
		valid bool
	}{
		{"ABC234", true},
		{"ABC231", false},
		{"abc234", false},
		{"ABC2345", false},
		{"../../x", false},
	} {
		if got := validID(test.id); got != test.valid {
			t.Errorf("validID(%q) = %v, want %v", test.id, got, test.valid)
		}
	}
}

func TestSafeFilename(t *testing.T) {
	if got := safeFilename(`..\secret\report.txt`); got != "report.txt" {
		t.Fatalf("safeFilename path traversal = %q", got)
	}
	if got := safeFilename("\x00\r\n"); got != "_" {
		t.Fatalf("safeFilename controls = %q", got)
	}
}

func TestDownloadIncludesAttachmentHeader(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "download-*")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString("hello"); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	previous := shares
	shares = map[string]Share{"ABC234": {ID: "ABC234", Expires: time.Now().Add(time.Hour).UnixMilli(), Files: []Asset{{Name: "报告 1.txt", Path: file.Name()}}}}
	t.Cleanup(func() { shares = previous })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/shares/ABC234/files/0", nil)
	download(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("download status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Disposition"); !strings.HasPrefix(got, "attachment;") || !strings.Contains(got, "filename*=") {
		t.Fatalf("unexpected Content-Disposition: %q", got)
	}
}

func TestDownloadAllIncludesEveryFile(t *testing.T) {
	dir := t.TempDir()
	paths := make([]string, 2)
	for i, content := range []string{"one", "two"} {
		file, err := os.CreateTemp(dir, "zip-*")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.WriteString(content); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		paths[i] = file.Name()
	}

	previous := shares
	shares = map[string]Share{"ABC234": {ID: "ABC234", Expires: time.Now().Add(time.Hour).UnixMilli(), Files: []Asset{{Name: "one.txt", Path: paths[0]}, {Name: "two.txt", Path: paths[1]}}}}
	t.Cleanup(func() { shares = previous })

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/shares/ABC234/zip", nil)
	downloadAll(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("zip status = %d, want 200", recorder.Code)
	}
	archive, err := zip.NewReader(bytes.NewReader(recorder.Body.Bytes()), int64(recorder.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	if len(archive.File) != 2 {
		t.Fatalf("zip entries = %d, want 2", len(archive.File))
	}
	for _, entry := range archive.File {
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		content, err := io.ReadAll(reader)
		_ = reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != "one" && string(content) != "two" {
			t.Errorf("unexpected ZIP content %q", content)
		}
	}
}
