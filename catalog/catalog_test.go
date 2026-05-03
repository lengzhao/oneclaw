package catalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_skipsReadmeMarkdown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# doc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "foo.readme.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "worker.md"), []byte("plain body"), 0o644); err != nil {
		t.Fatal(err)
	}
	cat, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Get("README") != nil || cat.Get("foo") != nil {
		t.Fatalf("readme entries loaded: %+v", cat.Agents)
	}
	if cat.Get("worker") == nil {
		t.Fatal("expected worker from worker.md")
	}
}
