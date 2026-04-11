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

func TestEnsureDirs_CreatesWriteRoots(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	lay.EnsureDirs()
	for _, d := range lay.WriteRoots() {
		if st, err := os.Stat(d); err != nil || !st.IsDir() {
			t.Fatalf("expected dir %s: err=%v", d, err)
		}
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

func TestIMHostMaintainLayout_DotOrDataRootAndEpisode(t *testing.T) {
	home := "/Users/x"
	ur := filepath.Join(home, DotDir)
	lay := IMHostMaintainLayout(ur, home)
	if !lay.HostUserData {
		t.Fatal("HostUserData")
	}
	if got := lay.DotOrDataRoot(); got != filepath.Clean(ur) {
		t.Fatalf("DotOrDataRoot = %q want %q", got, ur)
	}
	date := "2026-04-11"
	wantEp := filepath.Join(ur, "memory", date+".md")
	if got := lay.EpisodeDailyPath(date); got != wantEp {
		t.Fatalf("EpisodeDailyPath = %q want %q", got, wantEp)
	}
}
