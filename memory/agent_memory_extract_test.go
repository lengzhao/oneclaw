package memory

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	lzservice "github.com/lengzhao/memory/service"
)

func TestProjectExtractDailyMarkdownPath(t *testing.T) {
	got := ProjectExtractDailyMarkdownPath(filepath.Join("proj", "memory"), "2026-05-10")
	want := filepath.Join("proj", "memory", "2026-05-10.md")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatExtractedMemoriesMarkdown(t *testing.T) {
	ref := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
	out := formatExtractedMemoriesMarkdown(agentMemoryExtractPostTurn, ref, []lzservice.ExtractedMemory{
		{Title: "t1", Namespace: "knowledge", Content: "body", Summary: "sum", Confidence: 0.9, Importance: 80, Reasoning: "because"},
	})
	if !strings.Contains(out, "Structured extract (post_turn)") {
		t.Fatalf("missing header: %s", out)
	}
	if !strings.Contains(out, "### t1 [knowledge]") || !strings.Contains(out, "body") {
		t.Fatalf("missing memory block: %s", out)
	}
}
