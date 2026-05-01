package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/openai/openai-go"
)

func TestTrySlashLocalTurn_StatusAndPaths(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	e.SessionID = "test-sid"
	e.UserDataRoot = "/tmp/udr"
	e.TranscriptPath = "/tmp/t.jsonl"

	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/status", ClientID: "cli"})
	if !ok {
		t.Fatal("expected local slash")
	}
	for _, want := range []string{"test-sid", "cli", "TranscriptPath:", "工作区会话 ID", "driver client_id"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("status reply missing %q:\n%s", want, reply)
		}
	}

	reply, ok = e.trySlashLocalTurn(bus.InboundMessage{Content: "/paths"})
	if !ok {
		t.Fatal("expected local slash")
	}
	if !strings.Contains(reply, "MemoryBase:") || !strings.Contains(reply, "Project:") {
		t.Fatalf("paths reply: %s", reply)
	}
}

func TestTrySlashLocalTurn_SessionSubcommand(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	e.SessionID = "x"
	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/session full"})
	if !ok || !strings.Contains(reply, "工作区会话 ID") || !strings.Contains(reply, "x") {
		t.Fatalf("got ok=%v reply=%q", ok, reply)
	}
}

func TestTrySlashLocalTurn_SessionShort_showsDriverEnvelope(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	e.SessionID = "ws-hex"
	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{
		Content:   "/session",
		ClientID:  "webchat-1",
		SessionID: "wc-tab-1",
	})
	if !ok {
		t.Fatal("expected local slash")
	}
	for _, want := range []string{
		"driver client_id: webchat-1",
		"driver session_id (bus.SessionID): wc-tab-1",
		"路由 session_id (send_message / OutboundMessage.To): wc-tab-1",
		"工作区会话 ID",
		"ws-hex",
	} {
		if !strings.Contains(reply, want) {
			t.Fatalf("reply missing %q:\n%s", want, reply)
		}
	}
}

func TestTrySlashLocalTurn_Stop(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/stop"})
	if !ok || !strings.Contains(reply, "/stop") {
		t.Fatalf("got ok=%v reply=%q", ok, reply)
	}
	_, ok = e.trySlashLocalTurn(bus.InboundMessage{Content: "/stop x"})
	if !ok {
		t.Fatal("expected local reply for /stop with args")
	}
}

func TestTrySlashLocalTurn_ResetClearsMessagesAndTranscript(t *testing.T) {
	tmp := t.TempDir()
	tp := filepath.Join(tmp, "t.json")
	wp := filepath.Join(tmp, "w.json")
	e := NewEngine(tmp, nil)
	e.TranscriptPath = tp
	e.WorkingTranscriptPath = wp
	e.Messages = []openai.ChatCompletionMessageParamUnion{openai.UserMessage("old")}
	e.Transcript = []openai.ChatCompletionMessageParamUnion{openai.UserMessage("old"), openai.AssistantMessage("gone")}

	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/reset"})
	if !ok || !strings.Contains(reply, "已清空") {
		t.Fatalf("got ok=%v reply=%q", ok, reply)
	}
	if len(e.Messages) != 0 || len(e.Transcript) != 0 {
		t.Fatalf("expected empty slices, messages=%d transcript=%d", len(e.Messages), len(e.Transcript))
	}
	raw, err := os.ReadFile(tp)
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := loop.UnmarshalMessages(raw)
	if err != nil || len(msgs) != 0 {
		t.Fatalf("transcript file: err=%v len=%d raw=%s", err, len(msgs), raw)
	}
	raw, err = os.ReadFile(wp)
	if err != nil {
		t.Fatal(err)
	}
	msgs, err = loop.UnmarshalMessages(raw)
	if err != nil || len(msgs) != 0 {
		t.Fatalf("working transcript file: err=%v len=%d raw=%s", err, len(msgs), raw)
	}
}

func TestTrySlashLocalTurn_ResetRejectsArgs(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	e.Messages = []openai.ChatCompletionMessageParamUnion{openai.UserMessage("keep")}
	_, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/reset all"})
	if !ok {
		t.Fatal("expected local reply")
	}
	if len(e.Messages) != 1 {
		t.Fatalf("messages should not clear when args invalid: %d", len(e.Messages))
	}
}

func TestTrySlashLocalTurn_RecallIsStub(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/recall reset"})
	if !ok {
		t.Fatal("expected local slash")
	}
	if !strings.Contains(reply, "recall") {
		t.Fatalf("reply: %q", reply)
	}
}
