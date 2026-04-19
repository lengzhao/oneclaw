package memory

import (
	"path/filepath"
	"testing"
)

func TestSessionWorkspaceRel_RepoStyleHasNoDotDir(t *testing.T) {
	if got := SessionWorkspaceRel(false, "exec_log", "123", "run.log"); got != filepath.Join("exec_log", "123", "run.log") {
		t.Fatalf("SessionWorkspaceRel = %q", got)
	}
}

func TestJoinSessionWorkspace_RepoStyleHasNoDotDir(t *testing.T) {
	cwd := "/tmp/proj"
	want := filepath.Join(cwd, "agents")
	if got := JoinSessionWorkspace(cwd, false, "agents"); got != want {
		t.Fatalf("JoinSessionWorkspace = %q; want %q", got, want)
	}
}
