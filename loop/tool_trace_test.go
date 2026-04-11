package loop

import (
	"strings"
	"testing"
)

func TestPreviewToolOut_headAndTailWhenLong(t *testing.T) {
	// Same shape as exec foreground: failure line first, then a long log body.
	long := "exec_failed: exit status 9 " + strings.Repeat("x", 300)
	got := previewToolOut(long)
	if !strings.Contains(got, "exec_failed") {
		t.Fatalf("failure prefix should stay in preview: %q", got)
	}
	if !strings.Contains(got, "…") {
		t.Fatalf("want ellipsis when output is long: %q", got)
	}
	longTail := strings.Repeat("y", 300) + " TRAIL_MARKER"
	got2 := previewToolOut(longTail)
	if !strings.Contains(got2, "TRAIL_MARKER") {
		t.Fatalf("tail of long one-line output should be visible: %q", got2)
	}
}

func TestTruncateRunes(t *testing.T) {
	if got := truncateRunes("αβγδ", 2); got != "αβ…" {
		t.Fatalf("got %q", got)
	}
	if got := truncateRunes("ab", 10); got != "ab" {
		t.Fatalf("got %q", got)
	}
}

func TestToolTraceSinkSnapshotNil(t *testing.T) {
	var s *ToolTraceSink
	if s.Snapshot() != nil {
		t.Fatal("nil sink should return nil snapshot")
	}
	s.Add(ToolTraceEntry{Name: "x"}) // no-op
}

func TestToolTraceSinkSnapshotCopy(t *testing.T) {
	s := &ToolTraceSink{}
	s.Add(ToolTraceEntry{Name: "a", OK: true})
	out := s.Snapshot()
	out[0].Name = "mutated"
	snap2 := s.Snapshot()
	if snap2[0].Name != "a" {
		t.Fatalf("snapshot not a copy: %q", snap2[0].Name)
	}
}
