package memory

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/openai/openai-go"
)

func TestAppendDialogHistoryPair(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	date := "2026-04-06"
	path := lay.DialogHistoryPath(date)
	u := openai.UserMessage("hello")
	a := openai.AssistantMessage("hi")
	if err := AppendDialogHistoryPair(lay, date, "", u, a); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := loop.UnmarshalMessages(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 {
		t.Fatalf("want 2 messages, got %d", len(msgs))
	}
	if err := AppendDialogHistoryPair(lay, date, "", openai.UserMessage("q2"), openai.AssistantMessage("a2")); err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path)
	msgs2, err := loop.UnmarshalMessages(data2)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs2) != 4 {
		t.Fatalf("want 4 messages after second append, got %d", len(msgs2))
	}
}

func TestAppendDialogHistoryPair_trimsOld(t *testing.T) {
	t.Parallel()
	oldCap := maxDialogHistoryMessages
	maxDialogHistoryMessages = 4
	t.Cleanup(func() { maxDialogHistoryMessages = oldCap })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	date := "2026-04-06"
	path := lay.DialogHistoryPath(date)
	for i := 0; i < 4; i++ {
		if err := AppendDialogHistoryPair(lay, date, "", openai.UserMessage("u"), openai.AssistantMessage("a")); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := loop.UnmarshalMessages(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 4 {
		t.Fatalf("want cap 4 messages, got %d", len(msgs))
	}
}

func TestDialogHistoryPath(t *testing.T) {
	t.Parallel()
	cwd := filepath.Join("/tmp", "proj")
	lay := DefaultLayout(cwd, "/home/x")
	got := lay.DialogHistoryPath("2026-04-06")
	want := filepath.Join(cwd, "memory", "2026-04-06", "dialog_history.json")
	if got != want {
		t.Fatalf("path\ngot  %q\nwant %q", got, want)
	}
}

func TestDialogHistoryPathForSession(t *testing.T) {
	t.Parallel()
	cwd := filepath.Join("/tmp", "proj")
	lay := DefaultLayout(cwd, "/home/x")
	got := lay.DialogHistoryPathForSession("2026-04-06", "abc123")
	want := filepath.Join(cwd, "memory", "2026-04-06", "abc123", "dialog_history.json")
	if got != want {
		t.Fatalf("path\ngot  %q\nwant %q", got, want)
	}
}

func TestAppendDialogHistoryPair_sessionScoped(t *testing.T) {
	t.Parallel()
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	date := "2026-04-06"
	sid := "sess_seg"
	path := lay.DialogHistoryPathForSession(date, sid)
	u := openai.UserMessage("hello")
	a := openai.AssistantMessage("hi")
	if err := AppendDialogHistoryPair(lay, date, sid, u, a); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
