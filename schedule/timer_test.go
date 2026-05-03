package schedule

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNextWakeDuration_dueAndFuture(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "jobs.json")
	now := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	future := now.Add(30 * time.Second).Unix()
	if err := Save(p, &File{Jobs: []Job{
		{ID: "a", Enabled: true, Kind: KindOnce, NextRunUnix: future, SessionSegment: "s", Prompt: "x"},
	}}); err != nil {
		t.Fatal(err)
	}
	d, ok := NextWakeDuration(p, now)
	if !ok || d <= 0 || d > 31*time.Second {
		t.Fatalf("want ~30s got ok=%v d=%v", ok, d)
	}

	past := now.Add(-time.Minute).Unix()
	if err := Save(p, &File{Jobs: []Job{
		{ID: "b", Enabled: true, Kind: KindOnce, NextRunUnix: past, SessionSegment: "s", Prompt: "y"},
	}}); err != nil {
		t.Fatal(err)
	}
	d2, ok2 := NextWakeDuration(p, now)
	if !ok2 || d2 != 0 {
		t.Fatalf("want due immediately got ok=%v d=%v", ok2, d2)
	}
}
