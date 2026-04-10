package memory

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
)

func TestBuildDailyLogLineToolSummary(t *testing.T) {
	line := buildDailyLogLine("hello", "world", []loop.ToolTraceEntry{
		{Name: "read_file", OK: true},
		{Name: "exec", OK: false, Err: "exit 1"},
	})
	if !strings.Contains(line, "| tools: ") {
		t.Fatalf("missing tools: %q", line)
	}
	if !strings.Contains(line, "read_file:ok") || !strings.Contains(line, "exec:err") {
		t.Fatalf("unexpected content: %q", line)
	}
}

func TestBuildDailyLogLineNoTools(t *testing.T) {
	line := buildDailyLogLine("a", "b", nil)
	if strings.Contains(line, "| tools:") {
		t.Fatalf("should not add tools section: %q", line)
	}
}
