package schedule

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/paths"
)

func TestLoadSave_roundtrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scheduled_jobs.json")

	f0 := &File{
		Version: 1,
		Jobs: []Job{
			{ID: "j1", Enabled: true, Kind: KindOnce, NextRunUnix: 42, SessionSegment: "s", Prompt: "hi"},
		},
	}
	if err := Save(p, f0); err != nil {
		t.Fatal(err)
	}
	f1, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(f1.Jobs) != 1 || f1.Jobs[0].ID != "j1" || f1.Jobs[0].Prompt != "hi" {
		t.Fatalf("got %+v", f1.Jobs)
	}
}

func TestPoller_onceJobDisables(t *testing.T) {
	dir := t.TempDir()
	p := paths.ScheduledJobsPath(dir)

	var fired bool
	poller := NewPoller(p, func(context.Context, Job) error {
		fired = true
		return nil
	})
	poller.Now = func() time.Time { return time.Unix(100, 0) }

	if err := Save(p, &File{Jobs: []Job{
		{ID: "x", Enabled: true, Kind: KindOnce, NextRunUnix: 99, SessionSegment: "s", Prompt: "p"},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := poller.Tick(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !fired {
		t.Fatal("expected job to fire")
	}
	f2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(f2.Jobs) != 0 {
		t.Fatalf("once job row should be compacted after fire: %+v", f2.Jobs)
	}
}
