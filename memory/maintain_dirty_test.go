package memory

import (
	"database/sql"
	"os"
	"testing"

	"github.com/lengzhao/oneclaw/rtopts"
)

func TestAppendMaintenanceSection_MarksRecallIndexDirty(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}
	setSQLiteRuntimeForTest(t, lay)

	memPath := lay.EpisodeDailyPath("2026-04-18")
	if err := os.WriteFile(memPath, []byte("existing digest\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, []string{memPath}); err != nil {
		t.Fatal(err)
	}

	section := "## Auto-maintained (2026-04-18)\n- new fact\n"
	if err := appendMaintenanceSection(lay, memPath, section, AuditSourcePostTurnMaintain); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", recallSQLitePath(lay))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var dirty int
	if err := db.QueryRow(`SELECT dirty FROM memory_index_meta WHERE path = ?`, memPath).Scan(&dirty); err != nil {
		t.Fatal(err)
	}
	if dirty != 1 {
		t.Fatalf("dirty = %d, want 1", dirty)
	}
}
