package statichttp

import (
	"bytes"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestParseChatMultipart_textAndTwoFiles(t *testing.T) {
	cwd := t.TempDir()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField(formFieldText, "hello")
	_ = mw.WriteField(formFieldLocale, "zh-CN")
	w1, err := mw.CreateFormFile(formFieldFiles, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w1.Write([]byte("alpha"))
	w2, err := mw.CreateFormFile(formFieldFile, "b.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w2.Write([]byte("beta"))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/api/chat", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	got, err := parseChatMultipart(cwd, req)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello" || got.Locale != "zh-CN" {
		t.Fatalf("got %+v", got)
	}
	if len(got.Attachments) != 2 {
		t.Fatalf("attachments=%d", len(got.Attachments))
	}
	dateBucket := regexp.MustCompile(`media/inbound/\d{4}-\d{2}-\d{2}/`)
	for i, want := range []struct{ name, pathSub, noText string }{
		{"a.txt", ".oneclaw/media/inbound/", ""},
		{"b.txt", ".oneclaw/media/inbound/", ""},
	} {
		a := got.Attachments[i]
		if a.Name != want.name || a.Text != want.noText {
			t.Fatalf("att %d: %+v", i, a)
		}
		if !strings.Contains(a.Path, want.pathSub) || !dateBucket.MatchString(a.Path) {
			t.Fatalf("att %d path %q", i, a.Path)
		}
		abs := filepath.Join(cwd, filepath.FromSlash(a.Path))
		raw, err := os.ReadFile(abs)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 && string(raw) != "alpha" {
			t.Fatalf("file0 %q", raw)
		}
		if i == 1 && string(raw) != "beta" {
			t.Fatalf("file1 %q", raw)
		}
	}
}

func TestParseChatMultipart_fileTooLarge(t *testing.T) {
	cwd := t.TempDir()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, err := mw.CreateFormFile(formFieldFiles, "big.bin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(bytes.Repeat([]byte("x"), maxUploadedFileBytes+1))
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/api/chat", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	_, err = parseChatMultipart(cwd, req)
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseChatMultipart_requiresCwd(t *testing.T) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	w, _ := mw.CreateFormFile(formFieldFiles, "a.txt")
	_, _ = w.Write([]byte("x"))
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/chat", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	_, err := parseChatMultipart("", req)
	if err == nil || !strings.Contains(err.Error(), "working directory") {
		t.Fatalf("err=%v", err)
	}
}
