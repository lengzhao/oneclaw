package builtin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/toolctx"
)

func TestSendMessageToolUsesTurnDefaults(t *testing.T) {
	var got bus.InboundMessage
	tctx := toolctx.New(t.TempDir(), context.Background())
	tctx.TurnInbound = bus.InboundMessage{
		ClientID:  "cli",
		Peer:      bus.Peer{ID: "thr1", Kind: "direct"},
		Sender:    bus.SenderInfo{PlatformID: "u1", CanonicalID: "u1", Platform: "ten1"},
		SessionID: "Cxyz",
	}
	tctx.SendMessage = func(_ context.Context, in bus.InboundMessage) error {
		got = in
		return nil
	}

	tool := SendMessageTool{}
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"hello"}`), tctx)
	if err != nil || out != "sent" {
		t.Fatalf("Execute: out=%q err=%v", out, err)
	}
	if got.ClientID != "cli" || got.Content != "hello" {
		t.Fatalf("got %+v", got)
	}
	if got.SessionID != "Cxyz" || got.Peer.ID != "thr1" || got.Sender.PlatformID != "u1" || got.Sender.Platform != "ten1" {
		t.Fatalf("routing defaults %+v", got)
	}
}

func TestSendMessageToolOverrideSessionID(t *testing.T) {
	var got bus.InboundMessage
	tctx := toolctx.New(t.TempDir(), context.Background())
	tctx.TurnInbound = bus.InboundMessage{ClientID: "cli", Peer: bus.Peer{ID: "browser"}, SessionID: "C1"}
	tctx.SendMessage = func(_ context.Context, in bus.InboundMessage) error {
		got = in
		return nil
	}

	tool := SendMessageTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"x","session_id":"wc-other"}`), tctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "wc-other" {
		t.Fatalf("expected session_id wc-other, got %q", got.SessionID)
	}
}

func TestSendMessageToolLegacySessionKeyMapsToSessionID(t *testing.T) {
	var got bus.InboundMessage
	tctx := toolctx.New(t.TempDir(), context.Background())
	tctx.TurnInbound = bus.InboundMessage{ClientID: "cli", SessionID: "C1"}
	tctx.SendMessage = func(_ context.Context, in bus.InboundMessage) error {
		got = in
		return nil
	}
	tool := SendMessageTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"x","session_key":"legacy-tab"}`), tctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.SessionID != "legacy-tab" {
		t.Fatalf("got SessionID %q", got.SessionID)
	}
}

func TestSendMessageToolErrors(t *testing.T) {
	tctx := toolctx.New(t.TempDir(), context.Background())
	tool := SendMessageTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"x"}`), tctx)
	if err == nil || !strings.Contains(err.Error(), "client_id") {
		t.Fatalf("want client_id error, got %v", err)
	}

	tctx.TurnInbound.ClientID = "cli"
	tctx.SendMessage = nil
	_, err = tool.Execute(context.Background(), json.RawMessage(`{"text":"x"}`), tctx)
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("want unavailable, got %v", err)
	}
}
