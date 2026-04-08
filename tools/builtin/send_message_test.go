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
		Channel: "cli",
		Peer:    bus.Peer{ID: "thr1"},
		Sender:  bus.SenderInfo{PlatformID: "u1", CanonicalID: "u1", Platform: "ten1"},
		ChatID:  "Cxyz",
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
	if got.Channel != "cli" || got.Content != "hello" {
		t.Fatalf("got %+v", got)
	}
	if got.Peer.ID != "thr1" || got.Sender.PlatformID != "u1" || got.Sender.Platform != "ten1" {
		t.Fatalf("routing defaults %+v", got)
	}
}

func TestSendMessageToolOverrideSessionKey(t *testing.T) {
	var got bus.InboundMessage
	tctx := toolctx.New(t.TempDir(), context.Background())
	tctx.TurnInbound = bus.InboundMessage{Channel: "cli", Peer: bus.Peer{ID: "a"}, ChatID: "C1"}
	tctx.SendMessage = func(_ context.Context, in bus.InboundMessage) error {
		got = in
		return nil
	}

	tool := SendMessageTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"x","session_key":"other"}`), tctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Peer.ID != "other" {
		t.Fatalf("expected peer id other, got %q", got.Peer.ID)
	}
}

func TestSendMessageToolErrors(t *testing.T) {
	tctx := toolctx.New(t.TempDir(), context.Background())
	tool := SendMessageTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"x"}`), tctx)
	if err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("want source error, got %v", err)
	}

	tctx.TurnInbound.Channel = "cli"
	tctx.SendMessage = nil
	_, err = tool.Execute(context.Background(), json.RawMessage(`{"text":"x"}`), tctx)
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("want unavailable, got %v", err)
	}
}
