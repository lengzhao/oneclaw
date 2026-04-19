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
	br, cleanup := testStartNoopBridge(t, []string{"ch1"}, func(msg *bus.OutboundMessage) {
		published = append(published, msg)
	})
	defer cleanup()

	eng := NewEngine(cwd, tools.NewRegistry())
	eng.Bridge = br

	err := eng.SendMessage(context.Background(), bus.InboundMessage{
		ClientID:   "ch1",
		SessionID:  "C123",
		MessageID:  "cron-1",
		Content:    "notify",
		MediaPaths: nil,
		Peer:       bus.Peer{Kind: "channel"},
		Sender:     bus.SenderInfo{PlatformID: "U1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	waitForOutboundDispatch(t, func() bool { return len(published) >= 1 })
	if len(published) != 1 {
		t.Fatalf("want 1 outbound, got %d", len(published))
	}
	msg := published[0]
	if msg.ClientID != "ch1" || msg.Text != "notify" {
		t.Fatalf("outbound %+v", msg)
	}
	if msg.To.SessionID != "C123" {
		t.Fatalf("chat id %+v", msg.To)
	}
}

func TestEngineSendMessage_recipientUserIDFromMetadata(t *testing.T) {
	cwd := t.TempDir()
	var published []*bus.OutboundMessage
	br, cleanup := testStartNoopBridge(t, []string{"ch1"}, func(msg *bus.OutboundMessage) {
		published = append(published, msg)
	})
	defer cleanup()

	eng := NewEngine(cwd, tools.NewRegistry())
	eng.Bridge = br
	in := bus.InboundMessage{
		ClientID:  "ch1",
		SessionID: "S1",
		Content:   "dm",
		Peer:      bus.Peer{Kind: "direct"},
		Metadata: map[string]string{
			MetadataKeyOutboundRecipientUserID: "U-remote",
		},
	}
	if err := eng.SendMessage(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	waitForOutboundDispatch(t, func() bool { return len(published) >= 1 })
	if len(published) != 1 {
		t.Fatalf("got %d", len(published))
	}
	if published[0].To.UserID != "U-remote" {
		t.Fatalf("To.UserID: %+v", published[0].To)
	}
}

func TestEngineSendMessageRequiresClientID(t *testing.T) {
	br, cleanup := testStartNoopBridge(t, []string{"x"}, nil)
	defer cleanup()
	eng := NewEngine(t.TempDir(), tools.NewRegistry())
	eng.Bridge = br
	err := eng.SendMessage(context.Background(), bus.InboundMessage{Content: "x", SessionID: "C1"})
	if err == nil || !strings.Contains(err.Error(), "ClientID") {
		t.Fatalf("expected ClientID error, got %v", err)
	}
}

func TestEngineSendMessageNoChat(t *testing.T) {
	br, cleanup := testStartNoopBridge(t, []string{"x"}, nil)
	defer cleanup()
	eng := NewEngine(t.TempDir(), tools.NewRegistry())
	eng.Bridge = br
	err := eng.SendMessage(context.Background(), bus.InboundMessage{ClientID: "x", Content: "hi"})
	if err == nil || !strings.Contains(err.Error(), "SessionID") {
		t.Fatalf("expected SessionID error, got %v", err)
	}
}
