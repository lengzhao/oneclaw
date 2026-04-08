package notify

import (
	"testing"

	"github.com/lengzhao/oneclaw/loop"
)

func TestToolCallEndData(t *testing.T) {
	m := ToolCallEndData(loop.ToolTraceEntry{
		Step:        1,
		ToolUseID:   "call_1",
		Name:        "read_file",
		OK:          true,
		DurationMs:  12,
		ArgsPreview: "{}",
	})
	if m["tool_use_id"] != "call_1" || m["name"] != "read_file" {
		t.Fatalf("%v", m)
	}
}
