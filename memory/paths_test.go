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

func TestProjectMemoryDir(t *testing.T) {
	cwd := "/tmp/proj"
	got := ProjectMemoryDir(cwd)
	want := filepath.Join(cwd, DotDir, "memory")
	if got != want {
		t.Fatalf("ProjectMemoryDir = %q; want %q", got, want)
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

func TestRecallSQLitePath_DefaultAndRelative(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	lay := Layout{MemoryBase: "/tmp/oneclaw-home"}

	if got := recallSQLitePath(lay); got != filepath.Join(lay.MemoryBase, "memory", "recall_index.sqlite") {
		t.Fatalf("default recallSQLitePath = %q", got)
	}

	s := rtopts.DefaultSnapshot()
	s.MemoryRecallSQLitePath = "custom/recall.sqlite"
	rtopts.Set(&s)
	if got := recallSQLitePath(lay); got != filepath.Join(lay.MemoryBase, "custom", "recall.sqlite") {
		t.Fatalf("relative recallSQLitePath = %q", got)
	}
}

func TestSessionDotLayout_ProjectAndFlat(t *testing.T) {
	home := "/Users/x"
	dot := filepath.Join(home, ".oneclaw", "sessions", "abc", ".oneclaw")
	lay := SessionDotLayout(dot, home)
	if !lay.HostUserData {
		t.Fatal("HostUserData")
	}
	wantProj := filepath.Join(dot, "memory")
	if lay.Project != wantProj {
		t.Fatalf("Project = %q want %q", lay.Project, wantProj)
	}
}

func TestLayoutForIMWorkspace(t *testing.T) {
	home := "/Users/x"
	ur := filepath.Join(home, ".oneclaw")
	ws := filepath.Join(ur, IMWorkspaceDirName)
	if lay := LayoutForIMWorkspace(ws, home, ur, true, ur); !lay.HostUserData {
		t.Fatal("shared root should be IM host layout")
	}
	if lay := LayoutForIMWorkspace(ws, home, ur, true, ur); lay.InstructionRoot != ur {
		t.Fatalf("InstructionRoot = %q want %q", lay.InstructionRoot, ur)
	}
	dot := filepath.Join(ur, "sessions", "s1", ".oneclaw")
	if lay := LayoutForIMWorkspace(dot, home, ur, true, ""); lay.Project != filepath.Join(dot, "memory") {
		t.Fatalf("session dot layout: %+v", lay)
	}
	tmp := t.TempDir()
	if lay := LayoutForIMWorkspace(tmp, home, "", false, ""); lay.HostUserData {
		t.Fatal("repo layout should not set HostUserData")
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
