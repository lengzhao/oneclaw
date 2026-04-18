package schedule

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
)

func TestFormatScheduledUserText(t *testing.T) {
	at := time.Date(2026, 4, 6, 12, 0, 5, 0, time.UTC)
	got := FormatScheduledUserText("remind", "job_1", at, "  do the thing  ")
	if !strings.Contains(got, "【定时器触发】") || !strings.Contains(got, "来源：定时任务") {
		t.Fatalf("missing header: %q", got)
	}
	if !strings.Contains(got, "触发时间（UTC）：2026-04-06T12:00:05Z") {
		t.Fatalf("missing time: %q", got)
	}
	if !strings.Contains(got, "remind（id=job_1）") {
		t.Fatalf("missing task line: %q", got)
	}
	if !strings.Contains(got, "do the thing") {
		t.Fatalf("missing body: %q", got)
	}
}

func TestValidateExactlyOneSchedule(t *testing.T) {
	_, err := (ScheduleSpec{}).Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = ScheduleSpec{AtRFC3339: "2027-01-01T00:00:00Z", EverySeconds: 1}.Validate()
	if err == nil {
		t.Fatal("expected error for two kinds")
	}
}

func TestAddEveryAndCollectDue(t *testing.T) {
	cwd := t.TempDir()
	spec, err := ScheduleSpec{EverySeconds: 1}.Validate()
	if err != nil {
		t.Fatal(err)
	}
	// Force immediate fire: write job with next run in the past
	path := Path(cwd)
	dir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	f := &File{Version: 1, Jobs: []Job{{
		ID:           "sj_test1",
		Name:         "t",
		Message:      "hello",
		Enabled:      true,
		TargetSource: "cli",
		Schedule:     spec,
		NextRun:      past,
	}}}
	if err := write(path, f); err != nil {
		t.Fatal(err)
	}
	d, err := CollectDue(cwd, "", false, "", "cli", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 1 || d[0].Text == "" {
		t.Fatalf("deliveries: %+v", d)
	}
	if !strings.Contains(d[0].Text, "【定时器触发】") || !strings.Contains(d[0].Text, "hello") {
		t.Fatalf("unexpected delivery text: %q", d[0].Text)
	}
	f2, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f2.Jobs) != 1 {
		t.Fatal("expected one job")
	}
	if f2.Jobs[0].LastRun == nil {
		t.Fatal("expected last_run")
	}
	if f2.Jobs[0].NextRun.IsZero() {
		t.Fatal("expected next_run set for every")
	}
}

func TestCollectDueAtRemovesJob(t *testing.T) {
	cwd := t.TempDir()
	past := time.Now().UTC().Add(-2 * time.Minute)
	spec, err := ScheduleSpec{AtRFC3339: past.Format(time.RFC3339)}.Validate()
	if err != nil {
		t.Fatal(err)
	}
	path := Path(cwd)
	dir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := &File{Version: 1, Jobs: []Job{{
		ID:           "sj_at1",
		Name:         "once",
		Message:      "ping",
		Enabled:      true,
		TargetSource: "cli",
		Schedule:     spec,
		NextRun:      past,
	}}}
	if err := write(path, f); err != nil {
		t.Fatal(err)
	}
	d, err := CollectDue(cwd, "", false, "", "cli", time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(d) != 1 {
		t.Fatalf("want 1 delivery, got %d", len(d))
	}
	f2, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f2.Jobs) != 0 {
		t.Fatalf("expected at job removed after fire, got %d jobs", len(f2.Jobs))
	}
}

func TestCompactDisabledJobsOnList(t *testing.T) {
	cwd := t.TempDir()
	path := Path(cwd)
	dir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := &File{Version: 1, Jobs: []Job{
		{ID: "a", Name: "on", Message: "m", Enabled: true, TargetSource: "cli", Schedule: ScheduleSpec{Kind: "every", EverySeconds: 3600}, NextRun: time.Now().UTC().Add(time.Hour)},
		{ID: "b", Name: "off", Message: "x", Enabled: false, TargetSource: "cli", Schedule: ScheduleSpec{Kind: "every", EverySeconds: 3600}, NextRun: time.Now().UTC().Add(time.Hour)},
	}}
	if err := write(path, f); err != nil {
		t.Fatal(err)
	}
	_, err := ListText(cwd, "", false, "")
	if err != nil {
		t.Fatal(err)
	}
	f2, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(f2.Jobs) != 1 || f2.Jobs[0].ID != "a" {
		t.Fatalf("expected disabled job pruned, got %+v", f2.Jobs)
	}
}

func TestAddAtFuture(t *testing.T) {
	cwd := t.TempDir()
	at := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	msg, err := Add(cwd, "", false, "", AddInput{
		Message: "x",
		Schedule: ScheduleSpec{
			AtRFC3339: at,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if msg == "" {
		t.Fatal("empty msg")
	}
}

func TestMergeScheduleInputs_atSeconds(t *testing.T) {
	spec, err := MergeScheduleInputs(ScheduleSpec{}, 120)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Kind != "at" {
		t.Fatalf("kind: %q", spec.Kind)
	}
	_, err = MergeScheduleInputs(ScheduleSpec{EverySeconds: 1}, 60)
	if err == nil {
		t.Fatal("expected error when two schedule kinds set")
	}
}

func TestNextWakeDuration(t *testing.T) {
	cwd := t.TempDir()
	spec, err := ScheduleSpec{EverySeconds: 3600}.Validate()
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().UTC().Add(-time.Minute)
	path := Path(cwd)
	dir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := &File{Version: 1, Jobs: []Job{{
		ID:           "sj_w1",
		Name:         "w",
		Message:      "m",
		Enabled:      true,
		TargetSource: "cli",
		Schedule:     spec,
		NextRun:      past,
	}}}
	if err := write(path, f); err != nil {
		t.Fatal(err)
	}
	d, ok := NextWakeDuration(cwd, "", false, "", "cli", time.Now().UTC())
	if !ok || d != 0 {
		t.Fatalf("want overdue (d=0), got d=%v ok=%v", d, ok)
	}
}

func TestAddAtPastRejected(t *testing.T) {
	cwd := t.TempDir()
	at := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	_, err := Add(cwd, "", false, "", AddInput{
		Message: "x",
		Schedule: ScheduleSpec{
			AtRFC3339: at,
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestJobsFilePathSharedVsIsolated(t *testing.T) {
	home := t.TempDir()
	ur := filepath.Join(home, ".oneclaw")
	sessionRoot := filepath.Join(ur, "sessions", "abc123")
	sessionWS := filepath.Join(sessionRoot, memory.IMWorkspaceDirName)
	if err := os.MkdirAll(sessionWS, 0o755); err != nil {
		t.Fatal(err)
	}
	shared := JobsFilePath(filepath.Join(ur, memory.IMWorkspaceDirName), ur, true, ur)
	if want := filepath.Join(ur, "scheduled_jobs.json"); shared != want {
		t.Fatalf("shared path: got %q want %q", shared, want)
	}
	isol := JobsFilePath(sessionWS, ur, true, sessionRoot)
	if want := filepath.Join(sessionRoot, "scheduled_jobs.json"); isol != want {
		t.Fatalf("isolated path: got %q want %q", isol, want)
	}
	legacyEmptyCWD := JobsFilePath("", ur, true, "")
	if legacyEmptyCWD != filepath.Join(ur, "scheduled_jobs.json") {
		t.Fatalf("legacy poller path: %q", legacyEmptyCWD)
	}
}
