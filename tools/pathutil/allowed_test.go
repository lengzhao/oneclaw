package pathutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveUnderAllowedRoots_cwd(t *testing.T) {
	cwd := t.TempDir()
	p, err := ResolveUnderAllowedRoots(cwd, nil, "sub/f.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(p, "sub/f.txt") {
		t.Fatalf("got %s", p)
	}
}

func TestResolveUnderAllowedRoots_memoryExtra(t *testing.T) {
	cwd := t.TempDir()
	mem := t.TempDir()
	want := filepath.Join(mem, "x.md")
	p, err := ResolveUnderAllowedRoots(cwd, []string{mem}, want)
	if err != nil {
		t.Fatal(err)
	}
	if p != want {
		t.Fatalf("got %s want %s", p, want)
	}
}

func TestResolveUnderAllowedRoots_reject(t *testing.T) {
	cwd := t.TempDir()
	mem := t.TempDir()
	_, err := ResolveUnderAllowedRoots(cwd, []string{mem}, "/etc/passwd")
	if err == nil {
		t.Fatal("expected error")
	}
}
