package tasks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
)

func TestPathForWorkspace(t *testing.T) {
	cwd := "/proj/repo"
	want := filepath.Join(cwd, memory.DotDir, "tasks.json")
	if got := PathForWorkspace(cwd, false); got != want {
		t.Fatalf("PathForWorkspace: got %q want %q", got, want)
	}
	dot := "/data/.oneclaw"
	wantFlat := filepath.Join(dot, "tasks.json")
	if got := PathForWorkspace(dot, true); got != wantFlat {
		t.Fatalf("PathForWorkspace flat: got %q want %q", got, wantFlat)
	}
}

func TestCreateAppendAndUpdate(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, false, []CreateInput{{Subject: "first", Status: "pending"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := Read(PathForWorkspace(dir, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Items) != 1 || f.Items[0].Subject != "first" || f.Items[0].Status != "pending" {
		t.Fatalf("unexpected item: %+v", f.Items)
	}
	id := f.Items[0].ID
	if id == "" {
		t.Fatal("expected generated id")
	}
	_, err = Create(dir, false, false, []CreateInput{{Subject: "second"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err = Read(PathForWorkspace(dir, false))
	if err != nil || len(f.Items) != 2 {
		t.Fatalf("want 2 items, got %v err %v", f.Items, err)
	}
	st := "completed"
	ev := "verified in unit test"
	_, err = Update(dir, false, id, UpdatePatch{Status: &st, CompletionEvidence: &ev})
	if err != nil {
		t.Fatal(err)
	}
	f, err = Read(PathForWorkspace(dir, false))
	if err != nil {
		t.Fatal(err)
	}
	if f.Items[0].Status != "completed" {
		t.Fatalf("want completed, got %q", f.Items[0].Status)
	}
}

func TestCreateReplace(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, false, []CreateInput{{Subject: "a"}, {Subject: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Create(dir, false, true, []CreateInput{{Subject: "only"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := Read(PathForWorkspace(dir, false))
	if err != nil || len(f.Items) != 1 || f.Items[0].Subject != "only" {
		t.Fatalf("replace failed: %+v err %v", f.Items, err)
	}
}

func TestReadMissing(t *testing.T) {
	dir := t.TempDir()
	f, err := Read(PathForWorkspace(dir, false))
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Items) != 0 {
		t.Fatalf("want empty, got %+v", f.Items)
	}
}

func TestDisabled(t *testing.T) {
	dir := t.TempDir()
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.DisableTasks = true
	rtopts.Set(&s)
	if _, err := Create(dir, false, false, []CreateInput{{Subject: "x"}}); err == nil {
		t.Fatal("expected error when disabled")
	}
	_, lines, _ := PromptTaskLines(dir, false)
	if len(lines) > 0 {
		t.Fatalf("prompt lines should be empty when disabled, got %q", lines)
	}
}

func TestPromptTaskLinesNonEmpty(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, false, []CreateInput{{Subject: "do thing", Status: "in_progress"}})
	if err != nil {
		t.Fatal(err)
	}
	_, lines, _ := PromptTaskLines(dir, false)
	joined := strings.Join(lines, "\n")
	if len(lines) == 0 || !strings.Contains(joined, "do thing") || !strings.Contains(joined, "in_progress") {
		t.Fatalf("unexpected lines: %q", joined)
	}
}

func TestUpdateNoFields(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, false, []CreateInput{{Subject: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	f, _ := Read(PathForWorkspace(dir, false))
	msg, err := Update(dir, false, f.Items[0].ID, UpdatePatch{})
	if err != nil {
		t.Fatal(err)
	}
	if msg != "no fields to update" {
		t.Fatalf("got %q", msg)
	}
}

func TestCompletedRequiresEvidence(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, false, []CreateInput{{Subject: "x", Status: "in_progress"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := Read(PathForWorkspace(dir, false))
	if err != nil {
		t.Fatal(err)
	}
	id := f.Items[0].ID
	st := "completed"
	_, err = Update(dir, false, id, UpdatePatch{Status: &st})
	if err == nil {
		t.Fatal("expected error without completion_evidence")
	}
	meta := map[string]string{"completion_evidence": "done via metadata"}
	_, err = Update(dir, false, id, UpdatePatch{Status: &st, Metadata: meta})
	if err != nil {
		t.Fatal(err)
	}
	f, err = Read(PathForWorkspace(dir, false))
	if err != nil || f.Items[0].Status != "completed" {
		t.Fatalf("want completed, got %+v err %v", f.Items, err)
	}
}

func TestCompletedIdempotentNoEvidence(t *testing.T) {
	dir := t.TempDir()
	ev := "once"
	_, err := Create(dir, false, false, []CreateInput{{Subject: "x", Status: "pending"}})
	if err != nil {
		t.Fatal(err)
	}
	f, _ := Read(PathForWorkspace(dir, false))
	id := f.Items[0].ID
	st := "completed"
	if _, err := Update(dir, false, id, UpdatePatch{Status: &st, CompletionEvidence: &ev}); err != nil {
		t.Fatal(err)
	}
	if _, err := Update(dir, false, id, UpdatePatch{Status: &st}); err != nil {
		t.Fatalf("second completed without new evidence should succeed: %v", err)
	}
}

func TestInvalidStatus(t *testing.T) {
	dir := t.TempDir()
	bad := "nope"
	_, err := Create(dir, false, false, []CreateInput{{Subject: "x", Status: bad}})
	if err == nil {
		t.Fatal("expected invalid status error")
	}
}
