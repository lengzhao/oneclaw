package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func TestSubmitUser_localSlashDoesNotPersistTranscript(t *testing.T) {
	stub := openaistub.New(t)
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.ChatTransport = "non_stream"
	s.DisableMemory = true
	s.MemoryBase = ""
	rtopts.Set(&s)

	tmp := t.TempDir()
	tp := filepath.Join(tmp, "slim.json")
	wp := filepath.Join(tmp, "work.json")

	eng := NewEngine(tmp, tools.NewRegistry())
	eng.MaxSteps = 4
	eng.Client = openai.NewClient(
		option.WithAPIKey("sk-test-stub"),
		option.WithBaseURL(stub.BaseURL()),
	)
	eng.TranscriptPath = tp
	eng.WorkingTranscriptPath = wp
	var out string
	eng.PublishOutbound = func(_ context.Context, msg *bus.OutboundMessage) error {
		if msg != nil {
			out = msg.Text
		}
		return nil
	}

	in := bus.InboundMessage{
		ClientID:  "cli",
		SessionID: "s1",
		Peer:      bus.Peer{Kind: "channel"},
		Content:   "/help",
	}
	if err := eng.SubmitUser(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if len(eng.Messages) != 0 || len(eng.Transcript) != 0 {
		t.Fatalf("expected empty messages/transcript, got messages=%d transcript=%d", len(eng.Messages), len(eng.Transcript))
	}
	if !strings.Contains(out, "/model") {
		t.Fatalf("expected help in outbound, got %q", out)
	}
	raw, err := os.ReadFile(tp)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(raw) != 0 {
		t.Fatalf("slim transcript should stay empty, got len=%d", len(raw))
	}
	raw, err = os.ReadFile(wp)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if len(raw) != 0 {
		t.Fatalf("working transcript should stay empty, got len=%d", len(raw))
	}

	stub.Enqueue(openaistub.CompletionStop("", "from_model"))
	in.Content = "hi"
	if err := eng.SubmitUser(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	raw, err = os.ReadFile(tp)
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := loop.UnmarshalMessages(raw)
	if err != nil || len(msgs) < 2 {
		t.Fatalf("expected normal turn in slim transcript: err=%v n=%d", err, len(msgs))
	}
}
