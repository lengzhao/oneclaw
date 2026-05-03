package schedule

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func scopeS1() JobBindingScope {
	return JobBindingScope{SessionSegment: "s1", ClientID: "", AgentID: ""}
}

func TestAddScheduleJob_rejectsBelowMinGranularity(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scheduled_jobs.json")
	_, err := AddScheduleJob(p, ToolAddInput{
		Message:        "x",
		SessionSegment: "s1",
		AtSeconds:      9,
	})
	if err == nil || !strings.Contains(err.Error(), "at_seconds") {
		t.Fatalf("want at_seconds error, got %v", err)
	}
	_, err = AddScheduleJob(p, ToolAddInput{
		Message:        "x",
		SessionSegment: "s1",
		EverySeconds:   5,
	})
	if err == nil || !strings.Contains(err.Error(), "every_seconds") {
		t.Fatalf("want every_seconds error, got %v", err)
	}
}

func TestAddScheduleJob_onceThenListRemove(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scheduled_jobs.json")

	sum, err := AddScheduleJob(p, ToolAddInput{
		Message:        "hello reminder",
		SessionSegment: "s1",
		AtSeconds:      3600,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sum, "sj_") {
		t.Fatalf("summary %q", sum)
	}

	f, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Jobs) != 1 || f.Jobs[0].Kind != KindOnce || f.Jobs[0].Prompt != "hello reminder" {
		t.Fatalf("job: %+v", f.Jobs)
	}
	id := f.Jobs[0].ID

	txt, err := ListScheduleJobsText(p, scopeS1())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(txt, id) || !strings.Contains(txt, "once") {
		t.Fatalf("list: %s", txt)
	}

	rm, err := RemoveScheduleJob(p, id, scopeS1())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rm, "removed") {
		t.Fatalf("rm %q", rm)
	}
	f2, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(f2.Jobs) != 0 {
		t.Fatalf("want empty after remove: %+v", f2.Jobs)
	}
}

func TestAddScheduleJob_cronExprJSONRoundtrip(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scheduled_jobs.json")
	_, err := AddScheduleJob(p, ToolAddInput{
		Message:        "tick",
		SessionSegment: "tab-1",
		ClientID:       "webchat-1",
		AgentID:        "default",
		CronExpr:       "@every 2m",
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(raw.Jobs[0])
	var j Job
	if err := json.Unmarshal(b, &j); err != nil {
		t.Fatal(err)
	}
	j.Normalize()
	if j.Kind != KindCron || !strings.Contains(j.CronExpr, "every") {
		t.Fatalf("cron job: %+v", j)
	}
}

func TestListRemove_jobBindingIsolation(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "scheduled_jobs.json")
	if _, err := AddScheduleJob(p, ToolAddInput{
		Message:        "a",
		SessionSegment: "sess-a",
		ClientID:       "c1",
		AgentID:        "agent-x",
		AtSeconds:      7200,
	}); err != nil {
		t.Fatal(err)
	}
	f, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	id := f.Jobs[0].ID

	other := JobBindingScope{SessionSegment: "sess-a", ClientID: "c1", AgentID: "agent-y"}
	if txt, err := ListScheduleJobsText(p, other); err != nil {
		t.Fatal(err)
	} else if strings.Contains(txt, id) {
		t.Fatalf("list should hide other agent's job: %s", txt)
	}

	if _, err := RemoveScheduleJob(p, id, other); err == nil {
		t.Fatal("expected remove to fail for wrong agent scope")
	}

	self := JobBindingScope{SessionSegment: "sess-a", ClientID: "c1", AgentID: "agent-x"}
	if _, err := RemoveScheduleJob(p, id, self); err != nil {
		t.Fatal(err)
	}
}
