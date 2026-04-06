package builtin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/toolctx"
)

func TestSendMessageToolUsesTurnDefaults(t *testing.T) {
	var got routing.Inbound
	tctx := toolctx.New(t.TempDir(), context.Background())
	tctx.TurnInbound = routing.Inbound{
		Source: "cli", SessionKey: "thr1", UserID: "u1", TenantID: "ten1",
		RawRef: map[string]any{"ts": "1.2"},
	}
	tctx.SendMessage = func(_ context.Context, in routing.Inbound) error {
		got = in
		return nil
	}

	tool := SendMessageTool{}
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"hello"}`), tctx)
	if err != nil || out != "sent" {
		t.Fatalf("Execute: out=%q err=%v", out, err)
	}
	if got.Source != "cli" || got.Text != "hello" {
		t.Fatalf("got %+v", got)
	}
	if got.SessionKey != "thr1" || got.UserID != "u1" || got.TenantID != "ten1" {
		t.Fatalf("routing defaults %+v", got)
	}
	m, ok := got.RawRef.(map[string]any)
	if !ok || m["ts"] != "1.2" {
		t.Fatalf("raw ref %+v", got.RawRef)
	}
}

func TestSendMessageToolNoRawRefWhenOverrideSession(t *testing.T) {
	var got routing.Inbound
	tctx := toolctx.New(t.TempDir(), context.Background())
	tctx.TurnInbound = routing.Inbound{Source: "cli", SessionKey: "a", RawRef: "secret"}
	tctx.SendMessage = func(_ context.Context, in routing.Inbound) error {
		got = in
		return nil
	}

	tool := SendMessageTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"x","session_key":"other"}`), tctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.RawRef != nil {
		t.Fatalf("expected no raw ref when targeting other session, got %#v", got.RawRef)
	}
}

func TestSendMessageToolErrors(t *testing.T) {
	tctx := toolctx.New(t.TempDir(), context.Background())
	tool := SendMessageTool{}
	_, err := tool.Execute(context.Background(), json.RawMessage(`{"text":"x"}`), tctx)
	if err == nil || !strings.Contains(err.Error(), "source") {
		t.Fatalf("want source error, got %v", err)
	}

	tctx.TurnInbound.Source = "cli"
	tctx.SendMessage = nil
	_, err = tool.Execute(context.Background(), json.RawMessage(`{"text":"x"}`), tctx)
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("want unavailable, got %v", err)
	}
}
