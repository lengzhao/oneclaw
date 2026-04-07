package tasks

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
)

func TestPath(t *testing.T) {
	cwd := "/proj/repo"
	want := filepath.Join(cwd, memory.DotDir, "tasks.json")
	if got := Path(cwd); got != want {
		t.Fatalf("Path: got %q want %q", got, want)
	}
}

func TestCreateAppendAndUpdate(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, []CreateInput{{Subject: "first", Status: "pending"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := Read(Path(dir))
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
	_, err = Create(dir, false, []CreateInput{{Subject: "second"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err = Read(Path(dir))
	if err != nil || len(f.Items) != 2 {
		t.Fatalf("want 2 items, got %v err %v", f.Items, err)
	}
	st := "completed"
	_, err = Update(dir, id, UpdatePatch{Status: &st})
	if err != nil {
		t.Fatal(err)
	}
	f, err = Read(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if f.Items[0].Status != "completed" {
		t.Fatalf("want completed, got %q", f.Items[0].Status)
	}
}

func TestCreateReplace(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, []CreateInput{{Subject: "a"}, {Subject: "b"}})
	if err != nil {
		t.Fatal(err)
	}
	_, err = Create(dir, true, []CreateInput{{Subject: "only"}})
	if err != nil {
		t.Fatal(err)
	}
	f, err := Read(Path(dir))
	if err != nil || len(f.Items) != 1 || f.Items[0].Subject != "only" {
		t.Fatalf("replace failed: %+v err %v", f.Items, err)
	}
}

func TestReadMissing(t *testing.T) {
	dir := t.TempDir()
	f, err := Read(Path(dir))
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
	if _, err := Create(dir, false, []CreateInput{{Subject: "x"}}); err == nil {
		t.Fatal("expected error when disabled")
	}
	if b := SystemBlock(dir); b != "" {
		t.Fatalf("system block should be empty when disabled, got %q", b)
	}
}

func TestSystemBlockNonEmpty(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, []CreateInput{{Subject: "do thing", Status: "in_progress"}})
	if err != nil {
		t.Fatal(err)
	}
	b := SystemBlock(dir)
	if b == "" || !strings.Contains(b, "do thing") || !strings.Contains(b, "in_progress") {
		t.Fatalf("unexpected block: %q", b)
	}
}

func TestUpdateNoFields(t *testing.T) {
	dir := t.TempDir()
	_, err := Create(dir, false, []CreateInput{{Subject: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	f, _ := Read(Path(dir))
	msg, err := Update(dir, f.Items[0].ID, UpdatePatch{})
	if err != nil {
		t.Fatal(err)
	}
	if msg != "no fields to update" {
		t.Fatalf("got %q", msg)
	}
}

func TestInvalidStatus(t *testing.T) {
	dir := t.TempDir()
	bad := "nope"
	_, err := Create(dir, false, []CreateInput{{Subject: "x", Status: bad}})
	if err == nil {
		t.Fatal("expected invalid status error")
	}
}
