package memory

import (
	"os"
	"path/filepath"
	"strings"
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

func TestCollectIncrementalDailyLogCorpus(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	p := DailyLogPath(dir, today)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	tOld := now.Add(-4 * time.Hour).Format(time.RFC3339)
	tMid := now.Add(-2 * time.Hour).Format(time.RFC3339)
	tNew := now.Add(-30 * time.Minute).Format(time.RFC3339)
	body := "" +
		"- " + tOld + " | user: old | assistant: x\n" +
		"- " + tMid + " | user: mid | assistant: y\n" +
		"- " + tNew + " | user: new | assistant: z\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	hw := now.Add(-90 * time.Minute).UTC()
	corpus, raw, maxL, maxOK := collectIncrementalDailyLogCorpus(dir, &hw, 8*time.Hour, 10, 50_000, 200_000)
	if !maxOK {
		t.Fatal("expected maxOK")
	}
	wantNew, _ := time.Parse(time.RFC3339, tNew)
	d := maxL.Sub(wantNew)
	if d < 0 {
		d = -d
	}
	if d > 2*time.Second {
		t.Fatalf("max line %v want %v", maxL, wantNew)
	}
	if raw < 20 {
		t.Fatalf("raw: %d", raw)
	}
	if !strings.Contains(corpus, "user: new") || strings.Contains(corpus, "user: mid") {
		t.Fatalf("unexpected corpus: %q", corpus)
	}
}

func TestScheduledMaintainStatePathAndMigrate(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	legacyDir := filepath.Join(cwd, DotDir, "memory", DotDir)
	if err := os.MkdirAll(legacyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	legacyFile := filepath.Join(legacyDir, scheduledMaintainStateFile)
	payload := []byte(`{"last_success_wall_utc":"2026-04-06T00:00:00Z"}` + "\n")
	if err := os.WriteFile(legacyFile, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	lay := DefaultLayout(cwd, home)
	if got := scheduledMaintainStatePath(lay); got != filepath.Join(cwd, DotDir, scheduledMaintainStateFile) {
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

func TestCollectIncrementalFirstRunLookback(t *testing.T) {
	dir := t.TempDir()
	today := time.Now().Format("2006-01-02")
	p := DailyLogPath(dir, today)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	oldTS := time.Now().UTC().Add(-3 * time.Hour).Format(time.RFC3339)
	newTS := time.Now().UTC().Add(-30 * time.Minute).Format(time.RFC3339)
	body := "- " + oldTS + " | user: a | assistant: b\n- " + newTS + " | user: c | assistant: d\n"
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	corpus, _, _, ok := collectIncrementalDailyLogCorpus(dir, nil, time.Hour, 20, 50_000, 200_000)
	if corpus == "" {
		t.Fatal("expected corpus")
	}
	if !ok {
		t.Fatal("expected max ok")
	}
	if !strings.Contains(corpus, "user: c") {
		t.Fatalf("missing new line: %q", corpus)
	}
}
