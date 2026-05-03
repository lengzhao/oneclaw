package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveUnderWorkspace_rootDot(t *testing.T) {
	root := t.TempDir()
	got, err := ResolveUnderWorkspace(root, "")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Clean(root)
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveUnderWorkspace_relative(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveUnderWorkspace(root, "a/b")
	if err != nil {
		t.Fatal(err)
	}
	if got != sub {
		t.Fatalf("got %q want %q", got, sub)
	}
}

func TestResolveUnderWorkspace_rejectsDotDot(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveUnderWorkspace(root, ".."); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ResolveUnderWorkspace(root, "x/../../../etc/passwd"); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveUnderWorkspace_rejectsAbsoluteOutside(t *testing.T) {
	root := t.TempDir()
	if _, err := ResolveUnderWorkspace(root, "/etc/passwd"); err == nil {
		t.Fatal("expected error")
	}
}
