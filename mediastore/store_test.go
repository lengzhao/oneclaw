package mediastore

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
)

func TestStoreBytesAndValidate(t *testing.T) {
	cwd := t.TempDir()
	rel, err := StoreBytes(cwd, "hello.txt", []byte("hi"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rel, "media/inbound/") {
		t.Fatal(rel)
	}
	if ok, _ := regexp.MatchString(`inbound/\d{4}-\d{2}-\d{2}/`, rel); !ok {
		t.Fatalf("expected YYYY-MM-DD bucket in path: %s", rel)
	}
	if err := ValidateRelPath(cwd, rel); err != nil {
		t.Fatal(err)
	}
	abs := filepath.Join(cwd, filepath.FromSlash(rel))
	b, err := os.ReadFile(abs)
	if err != nil || string(b) != "hi" {
		t.Fatalf("%v %q", err, b)
	}
}

func TestValidateRelPath_rejectsEscape(t *testing.T) {
	cwd := t.TempDir()
	_ = os.MkdirAll(filepath.Join(cwd, memory.DotDir, "media", "inbound"), 0o755)
	if err := ValidateRelPath(cwd, "../outside"); err == nil {
		t.Fatal("expected error")
	}
}
