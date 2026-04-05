package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCollectRecentDailyLogs(t *testing.T) {
	dir := t.TempDir()
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	today := time.Now().Format("2006-01-02")
	yPath := DailyLogPath(dir, yesterday)
	tPath := DailyLogPath(dir, today)
	for _, p := range []string{filepath.Dir(yPath), filepath.Dir(tPath)} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(yPath, []byte(strings.Repeat("y", 250)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tPath, []byte(strings.Repeat("t", 250)), 0o644); err != nil {
		t.Fatal(err)
	}
	s, raw := collectRecentDailyLogs(dir, today, 3, 200, 10_000, 50_000)
	if raw < 400 {
		t.Fatalf("raw bytes: %d", raw)
	}
	if !strings.Contains(s, "### Daily log "+today) || !strings.Contains(s, "### Daily log "+yesterday) {
		t.Fatalf("missing sections: %s", s)
	}
}

func TestCollectProjectTopicExcerpts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "alpha.md"), []byte("topic alpha content"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := collectProjectTopicExcerpts(dir, 8, 100, 5000)
	if !strings.Contains(s, "alpha.md") || !strings.Contains(s, "topic alpha") {
		t.Fatalf("got %q", s)
	}
}
