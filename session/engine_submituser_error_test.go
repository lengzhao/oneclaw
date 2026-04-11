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
	eng := NewEngine(t.TempDir(), tools.NewRegistry())
	eng.MaxSteps = 4
	eng.Client = openai.NewClient(
		option.WithAPIKey("sk-test-stub"),
		option.WithBaseURL(stub.BaseURL()),
	)
	eng.PublishOutbound = func(_ context.Context, msg *bus.OutboundMessage) error {
		published = append(published, msg)
		return nil
	}

	in := bus.InboundMessage{
		Channel:   "cli",
		ChatID:    "C1",
		MessageID: "m1",
		Content:   "hi",
		Peer:      bus.Peer{Kind: "channel"},
	}
	err := eng.SubmitUser(context.Background(), in)
	if err == nil {
		t.Fatal("expected error (empty stub response queue)")
	}
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
	if published[0].ClientID != "cli" || published[0].To.ChatID != "C1" {
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
	eng := NewEngine(t.TempDir(), tools.NewRegistry())
	eng.MaxSteps = 4
	eng.Client = openai.NewClient(
		option.WithAPIKey("sk-test-stub"),
		option.WithBaseURL(stub.BaseURL()),
	)
	eng.PublishOutbound = func(_ context.Context, msg *bus.OutboundMessage) error {
		published = append(published, msg)
		return nil
	}

	err := eng.SubmitUser(context.Background(), bus.InboundMessage{Content: "hi"})
	if err == nil {
		t.Fatal("expected error")
	}
	if len(published) != 0 {
		t.Fatalf("without Channel/ChatID no outbound: got %+v", published)
	}
}
