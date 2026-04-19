package session

import (
	"context"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func TestSubmitUser_publishesOutboundOnFailure(t *testing.T) {
	stub := openaistub.New(t)
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.ChatTransport = "non_stream"
	s.DisableMemory = true
	s.MemoryBase = ""
	rtopts.Set(&s)

	var published []*bus.OutboundMessage
	cleanup := testStartNoopBridge(t, []string{"cli"}, func(msg *bus.OutboundMessage) {
		published = append(published, msg)
	})
	defer cleanup()

	eng := NewEngine(t.TempDir(), tools.NewRegistry())
	eng.MaxSteps = 4
	eng.Client = openai.NewClient(
		option.WithAPIKey("sk-test-stub"),
		option.WithBaseURL(stub.BaseURL()),
	)

	in := bus.InboundMessage{
		ClientID:  "cli",
		SessionID: "C1",
		MessageID: "m1",
		Content:   "hi",
		Peer:      bus.Peer{Kind: "channel"},
	}
	err := eng.SubmitUser(context.Background(), in)
	if err == nil {
		t.Fatal("expected error (empty stub response queue)")
	}
	waitForOutboundDispatch(t, func() bool { return len(published) >= 1 })
	if len(published) != 1 {
		t.Fatalf("want 1 error outbound, got %d (err=%v)", len(published), err)
	}
	body := published[0].Text
	if !strings.Contains(body, "处理失败") {
		t.Fatalf("outbound should explain failure in Chinese: %q", body)
	}
	if !strings.Contains(body, err.Error()) {
		t.Fatalf("outbound should include error text: outbound=%q err=%v", body, err)
	}
	if published[0].ClientID != "cli" || published[0].To.SessionID != "C1" {
		t.Fatalf("addressing: %+v", published[0])
	}
}

func TestSubmitUser_errorOutboundSkippedWithoutAddressing(t *testing.T) {
	stub := openaistub.New(t)
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.ChatTransport = "non_stream"
	s.DisableMemory = true
	s.MemoryBase = ""
	rtopts.Set(&s)

	var published []*bus.OutboundMessage
	// No clawbridge default bridge: PublishOutbound returns ErrNotInitialized; user-visible error line is skipped.
	eng := NewEngine(t.TempDir(), tools.NewRegistry())
	eng.MaxSteps = 4
	eng.Client = openai.NewClient(
		option.WithAPIKey("sk-test-stub"),
		option.WithBaseURL(stub.BaseURL()),
	)

	err := eng.SubmitUser(context.Background(), bus.InboundMessage{Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(published) != 0 {
		t.Fatalf("without ClientID/SessionID no outbound: got %+v", published)
	}
}
