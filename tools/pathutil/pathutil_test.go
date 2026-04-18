package pathutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUnderRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveUnderRoot(root, filepath.Join("a", "b"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Clean(got) != filepath.Clean(sub) {
		t.Fatalf("got %q want %q", got, sub)
	}
	_, err = ResolveUnderRoot(root, "..")
	if err == nil {
		t.Fatal("expected escape error")
	}
}

func TestIsUnderRoot(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b.txt")
	if err := os.MkdirAll(filepath.Dir(sub), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sub, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsUnderRoot(root, sub) {
		t.Fatal("expected under root")
	}
	out := filepath.Join(root, "..", "outside")
	if IsUnderRoot(root, out) {
		t.Fatal("expected outside root")
	}
}
