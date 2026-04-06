package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-113 /help 不调用模型
func TestE2E_113_SlashHelpSkipsModel(t *testing.T) {
	stub := openaistub.New(t)
	e2eEnvMinimal(t, stub)
	e := newStubEngine(t, t.TempDir())
	err := e.SubmitUser(context.Background(), routing.Inbound{Text: "/help", Source: "cli"})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(stub.ChatRequestBodies()); n != 0 {
		t.Fatalf("expected no chat/completions calls, got %d", n)
	}
	last := loop.LastAssistantDisplay(e.Messages)
	if !strings.Contains(last, "/model") {
		t.Fatalf("expected help body, got %q", last)
	}
}

// E2E-114 入站 meta + 附件进入 user 历史
func TestE2E_114_InboundMetaAndAttachmentInHistory(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub)
	e := newStubEngine(t, t.TempDir())
	err := e.SubmitUser(context.Background(), routing.Inbound{
		Text:       "see file",
		Source:     "http",
		SessionKey: "thr1",
		Locale:     "zh-CN",
		Attachments: []routing.Attachment{
			{Name: "f.txt", MIME: "text/plain", Text: "PAYLOAD_INLINE_MUST_NOT_APPEAR"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := concatUserText(e.Messages)
	if !strings.Contains(s, "<inbound-context>") || !strings.Contains(s, "session_key:") {
		t.Fatalf("missing inbound meta in:\n%s", s)
	}
	if !strings.Contains(s, "[Attachment: f.txt") || !strings.Contains(s, "read_file") || !strings.Contains(s, ".oneclaw/media/inbound") {
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
	e := newStubEngine(t, t.TempDir())
	err := e.SubmitUser(context.Background(), routing.Inbound{
		Text: "   ",
		Attachments: []routing.Attachment{
			{Name: "note.md", Text: "hello"},
		},
	})
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
	if !strings.Contains(s, ".oneclaw/media/inbound") {
		t.Fatalf("expected media path in:\n%s", s)
	}
	var found string
	for _, line := range strings.Split(s, "\n") {
		if strings.Contains(line, ".oneclaw/media/inbound") {
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
