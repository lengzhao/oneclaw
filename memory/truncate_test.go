package memory

import (
	"strings"
	"testing"
)

func TestTruncateEntrypointContent_noTruncation(t *testing.T) {
	s := "# hi\n- a\n"
	got := TruncateEntrypointContent(s)
	if got.WasLineTruncated || got.WasByteTruncated {
		t.Fatal("unexpected truncation")
	}
	if got.Content != strings.TrimSpace(s) {
		t.Fatalf("content %q", got.Content)
	}
}

func TestTruncateEntrypointContent_lineCap(t *testing.T) {
	var lines []string
	for i := 0; i < MaxEntrypointLines+3; i++ {
		lines = append(lines, "x")
	}
	raw := strings.Join(lines, "\n")
	got := TruncateEntrypointContent(raw)
	if !got.WasLineTruncated {
		t.Fatal("expected line truncation")
	}
	if !strings.Contains(got.Content, "WARNING") {
		t.Fatal("missing warning")
	}
}
