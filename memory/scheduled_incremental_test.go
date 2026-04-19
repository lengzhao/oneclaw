package memory

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseDailyLogLineTime(t *testing.T) {
	ts, ok := parseDailyLogLineTime("- 2026-04-06T12:30:45Z | user: hi | assistant: ok")
	if !ok {
		t.Fatal("expected ok")
	}
	if ts.UTC().Format(time.RFC3339) != "2026-04-06T12:30:45Z" {
		t.Fatalf("got %v", ts)
	}
	if _, ok := parseDailyLogLineTime("nope"); ok {
		t.Fatal("expected false")
	}
}

func TestScheduledMaintainStatePathAndMigrate(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	legacyDir := filepath.Join(cwd, "memory", DotDir)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(legacyDir, scheduledMaintainStateFile)
	payload := []byte(`{"last_success_wall_utc":"2026-04-06T00:00:00Z"}` + "\n")
	if err := os.WriteFile(legacyFile, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	lay := DefaultLayout(cwd, home)
	if got := scheduledMaintainStatePath(lay); got != filepath.Join(cwd, scheduledMaintainStateFile) {
		t.Fatalf("scheduledMaintainStatePath = %q", got)
	}

	migrateScheduledMaintainState(lay)
	newPath := scheduledMaintainStatePath(lay)
	b, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read new state: %v", err)
	}
	if string(b) != string(payload) {
		t.Fatalf("content mismatch")
	}
	if _, err := os.Stat(legacyFile); !os.IsNotExist(err) {
		t.Fatalf("legacy file should be gone: %v", err)
	}
}

func TestScheduledMaintainStatePath_IMHostLayout(t *testing.T) {
	home := t.TempDir()
	ur := filepath.Join(home, DotDir)
	lay := IMHostMaintainLayout(ur, home)
	want := filepath.Join(ur, scheduledMaintainStateFile)
	if got := scheduledMaintainStatePath(lay); got != want {
		t.Fatalf("scheduledMaintainStatePath = %q want %q", got, want)
	}
}
