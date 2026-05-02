//go:build e2e

package e2e_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/openai/openai-go"
)

// E2E-103 语义 compact：全局预算裁剪时在发往模型的首条请求 user 侧出现 compact_boundary 摘要块。
func TestE2E_103_SemanticCompactInChatRequest(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "e2e103 done"))
	e2eEnvMinimal(t, stub)
	s := rtopts.Current()
	s.Budget = budget.Global{
		MaxPromptBytes:      52_000,
		MinTailMessages:     4,
		HistoryMaxBytes:     0,
		SystemExtraMaxBytes: 0,
		AgentMdMaxBytes:     0,
		SkillIndexBytes:     0,
		InheritedMessages:   0,
	}
	s.DisableSemanticCompact = false
	rtopts.Set(&s)

	cwd := t.TempDir()
	ctx := context.Background()
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, 160)
	for range 150 {
		msgs = append(msgs, openai.UserMessage(strings.Repeat("q", 920)))
	}

	e := newStubEngine(t, stub, cwd)
	e.Messages = msgs
	if err := e.SubmitUser(ctx, stubInbound("E2E103_FINAL_USER")); err != nil {
		t.Fatal(err)
	}

	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatal("expected at least one completion request")
	}
	concat, err := openaistub.ChatRequestUserTextConcat(bodies[0])
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	if !strings.Contains(concat, "compact_boundary") {
		t.Fatalf("expected compact_boundary in user payload, got (prefix):\n%s", concat[:min(1200, len(concat))])
	}
}

// E2E-104 开启 disable_semantic_compact 时仅裁剪：首请求 user 文本不出现 compact_boundary（仍可能因预算丢头）。
func TestE2E_104_SemanticCompactDisabledNoBoundaryTag(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "e2e104 done"))
	e2eEnvMinimal(t, stub)
	s := rtopts.Current()
	s.Budget = budget.Global{
		MaxPromptBytes:      52_000,
		MinTailMessages:     4,
		HistoryMaxBytes:     0,
		SystemExtraMaxBytes: 0,
		AgentMdMaxBytes:     0,
		SkillIndexBytes:     0,
		InheritedMessages:   0,
	}
	s.DisableSemanticCompact = true
	rtopts.Set(&s)

	cwd := t.TempDir()
	ctx := context.Background()
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, 160)
	for range 150 {
		msgs = append(msgs, openai.UserMessage(strings.Repeat("r", 920)))
	}

	e := newStubEngine(t, stub, cwd)
	e.Messages = msgs
	if err := e.SubmitUser(ctx, stubInbound("E2E104_FINAL_USER")); err != nil {
		t.Fatal(err)
	}

	bodies := stub.ChatRequestBodies()
	concat, err := openaistub.ChatRequestUserTextConcat(bodies[0])
	if err != nil {
		t.Fatalf("parse request: %v", err)
	}
	if strings.Contains(concat, "compact_boundary") {
		t.Fatalf("did not expect compact_boundary when disabled, got (prefix):\n%s", concat[:min(1200, len(concat))])
	}
}
