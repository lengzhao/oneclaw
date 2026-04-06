package session

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

type sendMsgCaptureSink struct {
	lines []string
}

func (c *sendMsgCaptureSink) Emit(_ context.Context, r routing.Record) error {
	b, _ := json.Marshal(r)
	c.lines = append(c.lines, string(b))
	return nil
}

func TestEngineSendMessage(t *testing.T) {
	cwd := t.TempDir()
	reg := routing.NewMapRegistry()
	var sink sendMsgCaptureSink
	reg.Register("ch1", &sink)

	eng := NewEngine(cwd, builtin.DefaultRegistry())
	eng.SinkRegistry = reg

	err := eng.SendMessage(context.Background(), routing.Inbound{
		Source:        "ch1",
		Text:          "notify",
		CorrelationID: "cron-1",
		Attachments:   []routing.Attachment{{Name: "n", MIME: "text/plain", Text: "inline"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sink.lines) != 1 {
		t.Fatalf("want 1 record, got %d: %v", len(sink.lines), sink.lines)
	}
	rec := sink.lines[0]
	if !strings.Contains(rec, `"kind":"text"`) || !strings.Contains(rec, "notify") {
		t.Fatalf("text record: %s", rec)
	}
	if !strings.Contains(rec, `"job_id":"cron-1"`) {
		t.Fatalf("job_id: %s", rec)
	}
	if !strings.Contains(rec, `"path"`) {
		t.Fatalf("expected persisted path in attachments: %s", rec)
	}
}

func TestEngineSendMessageRequiresSource(t *testing.T) {
	eng := NewEngine(t.TempDir(), builtin.DefaultRegistry())
	eng.SinkRegistry = routing.NewMapRegistry()
	err := eng.SendMessage(context.Background(), routing.Inbound{Text: "x"})
	if err == nil || !strings.Contains(err.Error(), "Source") {
		t.Fatalf("expected Source error, got %v", err)
	}
}

func TestEngineSendMessageNoSink(t *testing.T) {
	eng := NewEngine(t.TempDir(), builtin.DefaultRegistry())
	eng.SinkRegistry = routing.NewMapRegistry()
	err := eng.SendMessage(context.Background(), routing.Inbound{Source: "missing", Text: "x"})
	if err == nil || !strings.Contains(err.Error(), "unknown source") {
		t.Fatalf("expected unknown source / no sink error, got %v", err)
	}
}
