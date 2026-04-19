//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-113 /help 不调用模型
func TestE2E_113_SlashHelpSkipsModel(t *testing.T) {
	stub := openaistub.New(t)
	e2eEnvMinimal(t, stub)
	var out string
	br, cleanup := e2eStartNoopBridge(t, []string{"cli"}, func(msg *bus.OutboundMessage) {
		if msg != nil {
			out = msg.Text
		}
	})
	defer cleanup()
	e := newStubEngine(t, stub, t.TempDir())
	e.Bridge = br
	in := bus.InboundMessage{Content: "/help", ClientID: "cli", SessionID: "t113", Peer: bus.Peer{Kind: "channel"}}
	err := e.SubmitUser(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(stub.ChatRequestBodies()); n != 0 {
		t.Fatalf("expected no chat/completions calls, got %d", n)
	}
	if len(e.Messages) != 0 {
		t.Fatalf("expected local slash not in engine Messages, got %d", len(e.Messages))
	}
	e2eWaitOutboundDispatch(t, func() bool { return strings.Contains(out, "/model") })
	if !strings.Contains(out, "/model") {
		t.Fatalf("expected help body in outbound, got %q", out)
	}
}

// E2E-116 /status 不调用模型
func TestE2E_116_SlashStatusSkipsModel(t *testing.T) {
	stub := openaistub.New(t)
	e2eEnvMinimal(t, stub)
	var out string
	br, cleanup := e2eStartNoopBridge(t, []string{"cli"}, func(msg *bus.OutboundMessage) {
		if msg != nil {
			out = msg.Text
		}
	})
	defer cleanup()
	e := newStubEngine(t, stub, t.TempDir())
	e.Bridge = br
	in := bus.InboundMessage{Content: "/status", ClientID: "cli", SessionID: "t116", Peer: bus.Peer{Kind: "channel"}}
	err := e.SubmitUser(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(stub.ChatRequestBodies()); n != 0 {
		t.Fatalf("expected no chat/completions calls, got %d", n)
	}
	if len(e.Messages) != 0 {
		t.Fatalf("expected local slash not in engine Messages, got %d", len(e.Messages))
	}
	e2eWaitOutboundDispatch(t, func() bool { return strings.Contains(out, "工作区会话 ID") })
	if !strings.Contains(out, "工作区会话 ID") {
		t.Fatalf("expected status body in outbound, got %q", out)
	}
}

// E2E-117 /stop 不调用模型
func TestE2E_117_SlashStopSkipsModel(t *testing.T) {
	stub := openaistub.New(t)
	e2eEnvMinimal(t, stub)
	var out string
	br, cleanup := e2eStartNoopBridge(t, []string{"cli"}, func(msg *bus.OutboundMessage) {
		if msg != nil {
			out = msg.Text
		}
	})
	defer cleanup()
	e := newStubEngine(t, stub, t.TempDir())
	e.Bridge = br
	in := bus.InboundMessage{Content: "/stop", ClientID: "cli", SessionID: "t117", Peer: bus.Peer{Kind: "channel"}}
	err := e.SubmitUser(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(stub.ChatRequestBodies()); n != 0 {
		t.Fatalf("expected no chat/completions calls, got %d", n)
	}
	if len(e.Messages) != 0 {
		t.Fatalf("expected local slash not in engine Messages, got %d", len(e.Messages))
	}
	e2eWaitOutboundDispatch(t, func() bool { return strings.Contains(out, "/stop") })
	if !strings.Contains(out, "/stop") {
		t.Fatalf("expected stop reply in outbound, got %q", out)
	}
}

// E2E-118 /reset 不调用模型，且清空此前回合在内存中的可见历史
func TestE2E_118_SlashResetSkipsModelAndClearsHistory(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "UNIQUE_FIRST_ASSISTANT_REPLY"))
	e2eEnvMinimal(t, stub)
	var lastOutbound string
	br, cleanup := e2eStartNoopBridge(t, []string{"cli"}, func(msg *bus.OutboundMessage) {
		if msg != nil && strings.TrimSpace(msg.Text) != "" {
			lastOutbound = msg.Text
		}
	})
	defer cleanup()
	e := newStubEngine(t, stub, t.TempDir())
	e.Bridge = br
	thread := bus.InboundMessage{ClientID: "cli", SessionID: "t118", Peer: bus.Peer{ID: "p1", Kind: "channel"}}
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "hello", ClientID: thread.ClientID, SessionID: thread.SessionID, Peer: thread.Peer}); err != nil {
		t.Fatal(err)
	}
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "/reset", ClientID: thread.ClientID, SessionID: thread.SessionID, Peer: thread.Peer}); err != nil {
		t.Fatal(err)
	}
	e2eWaitOutboundDispatch(t, func() bool { return strings.Contains(lastOutbound, "已清空") })
	if n := len(stub.ChatRequestBodies()); n != 1 {
		t.Fatalf("expected one chat/completions call (first turn only), got %d", n)
	}
	vis := loop.ToUserVisibleMessages(e.Messages)
	joinedBytes, err := json.Marshal(vis)
	if err != nil {
		t.Fatal(err)
	}
	joined := string(joinedBytes)
	if strings.Contains(joined, "UNIQUE_FIRST_ASSISTANT_REPLY") {
		t.Fatalf("expected prior assistant text cleared from visible messages, got:\n%s", joined)
	}
	if strings.Contains(joined, "/reset") {
		t.Fatalf("expected /reset not persisted in visible messages, got:\n%s", joined)
	}
	if len(vis) != 0 {
		t.Fatalf("expected empty visible messages after reset, got %d: %s", len(vis), joined)
	}
	if !strings.Contains(lastOutbound, "已清空") {
		t.Fatalf("expected reset confirmation in outbound, got %q", lastOutbound)
	}
}

// E2E-114 入站 meta 进入首轮 API 请求；附件路径留在折叠后的 Messages（inbound 不常驻内存以省 token）
func TestE2E_114_InboundMetaAndAttachmentInHistory(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub)
	e := newStubEngine(t, stub, t.TempDir())
	err := e.SubmitUser(context.Background(), inboundWithPersistedAttachments(t, e.CWD, "see file", "http", "thr1", []session.Attachment{
		{Name: "f.txt", MIME: "text/plain", Text: "PAYLOAD_INLINE_MUST_NOT_APPEAR"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatal("expected chat request")
	}
	reqText, err := openaistub.ChatRequestUserTextConcat(bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reqText, "<inbound-context>") || !strings.Contains(reqText, "session_key:") || !strings.Contains(reqText, "workspace_session_id:") {
		t.Fatalf("missing inbound meta in first request:\n%s", reqText)
	}
	s := concatUserText(e.Messages)
	if strings.Contains(s, "<inbound-context>") {
		t.Fatalf("inbound meta should not remain in collapsed Messages:\n%s", s)
	}
	if !strings.Contains(s, "f.txt") || !strings.Contains(s, "read_file") || !strings.Contains(s, "media/inbound") {
		t.Fatalf("expected stored path + read_file hint, got:\n%s", s)
	}
	if strings.Contains(s, "PAYLOAD_INLINE_MUST_NOT_APPEAR") {
		t.Fatalf("attachment bytes must not be inlined for the model:\n%s", s)
	}
}

// E2E-115 空正文 + 附件合法
func TestE2E_115_EmptyTextWithAttachmentAccepted(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "read"))
	e2eEnvMinimal(t, stub)
	e := newStubEngine(t, stub, t.TempDir())
	err := e.SubmitUser(context.Background(), inboundWithPersistedAttachments(t, e.CWD, "   ", "", "", []session.Attachment{
		{Name: "note.md", Text: "hello"},
	}))
	if err != nil {
		t.Fatal(err)
	}
	if loop.LastAssistantDisplay(e.Messages) != "read" {
		t.Fatalf("got %q", loop.LastAssistantDisplay(e.Messages))
	}
	s := concatUserText(e.Messages)
	if strings.Contains(s, "hello") {
		t.Fatalf("file contents must not appear inline in user messages:\n%s", s)
	}
	if !strings.Contains(s, "media/inbound") {
		t.Fatalf("expected media path in:\n%s", s)
	}
	var found string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, "media/inbound") {
			found = strings.TrimSpace(line)
			break
		}
	}
	if found == "" {
		t.Fatal("no path line")
	}
	raw, err := os.ReadFile(filepath.Join(e.CWD, filepath.FromSlash(found)))
	if err != nil || string(raw) != "hello" {
		t.Fatalf("read stored file: err=%v body=%q", err, raw)
	}
}
