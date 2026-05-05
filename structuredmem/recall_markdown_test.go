package structuredmem

import (
	"context"
	"strings"
	"testing"
	"time"

	lzmem "github.com/lengzhao/memory"
)

func TestFormatRecallMarkdown_hit(t *testing.T) {
	dir := t.TempDir()
	db, err := OpenDB(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = lzmem.Close(db) }()

	ctx := lzmem.WithIsolation(context.Background(), "default", "sess-x", "sess-x", "default-agent")
	svc := lzmem.NewMemoryService(db)
	if _, err := svc.Remember(ctx, lzmem.RememberRequest{
		NamespaceType: lzmem.NamespaceKnowledge,
		Title:         "favoriteColorBlue",
		Content:       "The user said their favorite color is blue.",
		Summary:       "likes blue",
		Confidence:    0.95,
		Importance:    3,
	}); err != nil {
		t.Fatal(err)
	}

	got := FormatRecallMarkdown(context.Background(), dir, "favorite color", "sess-x", "default-agent", 4000)
	if !strings.Contains(got, "favoriteColorBlue") || !strings.Contains(got, "Structured memory") {
		t.Fatalf("unexpected recall markdown:\n%s", got)
	}
}

func TestFormatExtractResultMarkdown(t *testing.T) {
	ts := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)
	got := FormatExtractResultMarkdown(ts, &lzmem.ExtractResult{
		Status:         "completed",
		ExtractionID:   "ext-1",
		TotalTokens:    42,
		ProcessingTime: 100,
		Memories: []lzmem.ExtractedMemory{
			{
				Namespace:  lzmem.NamespaceProfile,
				Title:      "Name",
				Summary:    "Call me Alex",
				Confidence: 0.9,
				Importance: 2,
			},
		},
	})
	if !strings.Contains(got, "ext-1") || !strings.Contains(got, "Call me Alex") {
		t.Fatalf("unexpected markdown:\n%s", got)
	}
}
