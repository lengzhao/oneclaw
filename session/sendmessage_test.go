package session

import (
	"context"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/tools"
)

func TestEngineSendMessage(t *testing.T) {
	cwd := t.TempDir()
	var published []*bus.OutboundMessage
	eng := NewEngine(cwd, tools.NewRegistry())
	eng.PublishOutbound = func(_ context.Context, msg *bus.OutboundMessage) error {
		published = append(published, msg)
		return nil
	}

	err := eng.SendMessage(context.Background(), bus.InboundMessage{
		Channel:       "ch1",
		ChatID:        "C123",
		MessageID:     "cron-1",
		Content:       "notify",
		MediaPaths:    nil,
		Peer:          bus.Peer{Kind: "channel"},
		Sender:        bus.SenderInfo{PlatformID: "U1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(published) != 1 {
		t.Fatalf("want 1 outbound, got %d", len(published))
	}
	msg := published[0]
	if msg.ClientID != "ch1" || msg.Text != "notify" {
		t.Fatalf("outbound %+v", msg)
	}
	if msg.To.ChatID != "C123" {
		t.Fatalf("chat id %+v", msg.To)
	}
}

func TestEngineSendMessageRequiresChannel(t *testing.T) {
	eng := NewEngine(t.TempDir(), tools.NewRegistry())
	eng.PublishOutbound = func(context.Context, *bus.OutboundMessage) error { return nil }
	err := eng.SendMessage(context.Background(), bus.InboundMessage{Content: "x", ChatID: "C1"})
	if err == nil || !strings.Contains(err.Error(), "Channel") {
		t.Fatalf("expected Channel error, got %v", err)
	}
}

func TestEngineSendMessageNoChat(t *testing.T) {
	eng := NewEngine(t.TempDir(), tools.NewRegistry())
	eng.PublishOutbound = func(context.Context, *bus.OutboundMessage) error { return nil }
	err := eng.SendMessage(context.Background(), bus.InboundMessage{Channel: "x", Content: "hi"})
	if err == nil || !strings.Contains(err.Error(), "ChatID") {
		t.Fatalf("expected ChatID error, got %v", err)
	}
}
