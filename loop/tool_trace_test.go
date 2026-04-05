package loop

import "testing"

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
