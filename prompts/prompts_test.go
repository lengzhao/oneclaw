package prompts

import (
	"testing"
)

func TestRenderCompactEnvelope(t *testing.T) {
	const kind = "compact_boundary"
	const ts = "2006-01-02T15:04:05Z"
	summary := "line"
	want := "[oneclaw:" + kind + " ts=" + ts + "]\n" +
		"Earlier conversation (omitted from context for byte budget). Heuristic recap — verify with tools if needed:\n\n" +
		summary + "\n\n[/oneclaw:" + kind + "]\n"
	got, err := Render(NameCompactEnvelope, map[string]string{
		"Kind": kind, "Timestamp": ts, "Summary": summary,
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRenderNilDataUsesEmptyStruct(t *testing.T) {
	_, err := Render(NameMainThreadSystem, map[string]any{
		"CWD": "/tmp", "Platform": "darwin", "Shell": "zsh", "TasksFilePath": "/tmp/tasks.json",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRenderEmptyName(t *testing.T) {
	_, err := Render("", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderUnknownTemplate(t *testing.T) {
	_, err := Render("does_not_exist", nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

