package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/rtopts"
)

func TestExpandTilde(t *testing.T) {
	home := "/Users/someone"
	cases := []struct {
		in, want string
	}{
		{"", ""},
		{"/abs", "/abs"},
		{"~", home},
		{"~/foo", filepath.Join(home, "foo")},
		{"~/a/b", filepath.Join(home, "a", "b")},
		{"~\\w", filepath.Join(home, "w")},
		{"~other", "~other"},
	}
	for _, tc := range cases {
		got := expandTilde(home, tc.in)
		if got != tc.want {
			t.Errorf("expandTilde(%q, %q) = %q; want %q", home, tc.in, got, tc.want)
		}
	}
}

func TestEnsureDefaultAgentMd_MigratesRootToDot(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	root := filepath.Join(cwd, AgentInstructionsFile)
	if err := os.WriteFile(root, []byte("legacy root content\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	EnsureDefaultAgentMd(lay)
	dot := filepath.Join(cwd, DotDir, AgentInstructionsFile)
	b, err := os.ReadFile(dot)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "legacy root content\n" {
		t.Fatalf("migrated content = %q", string(b))
	}
}

func TestEnsureDefaultAgentMd_CreatesDotOneclaw(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	EnsureDefaultAgentMd(lay)
	dot := filepath.Join(cwd, DotDir, AgentInstructionsFile)
	b, err := os.ReadFile(dot)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 20 {
		t.Fatalf("unexpected stub: %q", b)
	}
	// Idempotent
	EnsureDefaultAgentMd(lay)
	b2, err := os.ReadFile(dot)
	if err != nil {
		t.Fatal(err)
	}
	if string(b2) != string(b) {
		t.Fatal("second call should not overwrite")
	}
}

func TestProjectMemoryMdPath(t *testing.T) {
	cwd := "/tmp/proj"
	got := ProjectMemoryMdPath(cwd)
	want := filepath.Join(cwd, DotDir, "memory", "MEMORY.md")
	if got != want {
		t.Fatalf("ProjectMemoryMdPath = %q; want %q", got, want)
	}
}

func TestMemoryBaseDir_ExpandsTildeInEnv(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	rtopts.Set(&rtopts.Snapshot{MemoryBase: "~/custom-base"})
	got := MemoryBaseDir("/Users/someone")
	want := filepath.Join("/Users/someone", "custom-base")
	if got != filepath.Clean(want) {
		t.Fatalf("MemoryBaseDir = %q; want %q", got, want)
	}
}
