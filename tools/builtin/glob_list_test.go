package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/toolctx"
)

func TestMatchPathPattern(t *testing.T) {
	cases := []struct {
		rel, pat string
		want     bool
	}{
		{"a.go", "*.go", true},
		{"a.txt", "*.go", false},
		{"src/a.go", "**/*.go", true},
		{"src/sub/a.go", "**/*.go", true},
		{"src/sub/a.txt", "**/*.go", false},
		{"x", "**", true},
		{"a/b/c", "**", true},
	}
	for _, tc := range cases {
		if got := matchPathPattern(tc.rel, tc.pat); got != tc.want {
			t.Errorf("matchPathPattern(%q,%q) = %v, want %v", tc.rel, tc.pat, got, tc.want)
		}
	}
}

func TestGlobTool_shallow(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tctx := toolctx.New(dir, context.Background())
	raw, _ := json.Marshal(map[string]string{"path": ".", "pattern": "*.go"})
	out, err := (GlobTool{}).Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != "a.go" {
		t.Fatalf("got %q", out)
	}
}

func TestGlobTool_recursive_skipsDotGit(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "src", "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "src", "pkg", "x.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "hooks", "x.go"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tctx := toolctx.New(dir, context.Background())
	raw, _ := json.Marshal(map[string]string{"path": ".", "pattern": "**/*.go"})
	out, err := (GlobTool{}).Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "src/pkg/x.go") {
		t.Fatalf("missing src/pkg/x.go in %q", out)
	}
	if strings.Contains(out, ".git") {
		t.Fatalf("should skip .git, got %q", out)
	}
}

func TestListDirTool_oneLevel(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	tctx := toolctx.New(dir, context.Background())
	raw, _ := json.Marshal(map[string]any{"path": "."})
	out, err := (ListDirTool{}).Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "f.txt") || !strings.Contains(out, "sub/") {
		t.Fatalf("got %q", out)
	}
}

func TestListDirTool_recursive(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a", "b", "c.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	tctx := toolctx.New(dir, context.Background())
	raw, _ := json.Marshal(map[string]any{"path": ".", "recursive": true})
	out, err := (ListDirTool{}).Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "a/") || !strings.Contains(out, "a/b/") || !strings.Contains(out, "a/b/c.txt") {
		t.Fatalf("got %q", out)
	}
}
