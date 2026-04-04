package routing

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

type captureSink struct {
	lines []string
}

func (c *captureSink) Emit(_ context.Context, r Record) error {
	b, _ := json.Marshal(r)
	c.lines = append(c.lines, string(b))
	return nil
}

func TestEmitterSeqAndKinds(t *testing.T) {
	var cap captureSink
	e := NewEmitter(&cap, "sess_x", "")
	ctx := context.Background()
	_ = e.Text(ctx, "hi")
	_ = e.ToolStart(ctx, "read_file")
	_ = e.ToolEnd(ctx, "read_file", true)
	_ = e.Done(ctx, true, "")

	if len(cap.lines) != 4 {
		t.Fatalf("want 4 emits, got %d", len(cap.lines))
	}
	if !strings.Contains(cap.lines[0], `"seq":1`) || !strings.Contains(cap.lines[0], `"kind":"text"`) {
		t.Fatalf("first record: %s", cap.lines[0])
	}
	if !strings.Contains(cap.lines[3], `"kind":"done"`) {
		t.Fatalf("last record: %s", cap.lines[3])
	}
}
