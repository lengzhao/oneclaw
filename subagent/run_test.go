package subagent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

func assistantWithToolCalls(ids ...string) openai.ChatCompletionMessageParamUnion {
	tcs := make([]openai.ChatCompletionMessageToolCallParam, 0, len(ids))
	for _, id := range ids {
		tcs = append(tcs, openai.ChatCompletionMessageToolCallParam{
			ID: id,
			Function: openai.ChatCompletionMessageToolCallFunctionParam{
				Name:      "run_agent",
				Arguments: "{}",
			},
		})
	}
	return openai.ChatCompletionMessageParamUnion{
		OfAssistant: &openai.ChatCompletionAssistantMessageParam{ToolCalls: tcs},
	}
}

func TestTrimInheritedParentMessages_dropsTrailingAssistantWithUnresolvedToolCalls(t *testing.T) {
	pending := assistantWithToolCalls("call_pending")
	src := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("hi"),
		openai.AssistantMessage("ok"),
		pending,
	}
	out := trimInheritedParentMessages(src, 32)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2: %+v", len(out), out)
	}
	if out[0].OfUser == nil || UserText(out[0]) != "hi" {
		t.Fatalf("first message: %+v", out[0])
	}
	if out[1].OfAssistant == nil || UserText(out[1]) != "ok" {
		t.Fatalf("second message: %+v", out[1])
	}
}

func UserText(m openai.ChatCompletionMessageParamUnion) string {
	if m.OfUser != nil {
		return m.OfUser.Content.OfString.Value
	}
	if m.OfAssistant != nil && m.OfAssistant.Content.OfString.Valid() {
		return m.OfAssistant.Content.OfString.Value
	}
	return ""
}

func TestTrimInheritedParentMessages_keepsCompleteToolRound(t *testing.T) {
	a := assistantWithToolCalls("c1", "c2")
	src := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("u"),
		a,
		openai.ToolMessage("out1", "c1"),
		openai.ToolMessage("out2", "c2"),
	}
	out := trimInheritedParentMessages(src, 32)
	if len(out) != 4 {
		t.Fatalf("len=%d want 4", len(out))
	}
}

func TestTrimInheritedParentMessages_dropsPartialToolBatch(t *testing.T) {
	a := assistantWithToolCalls("c1", "c2")
	src := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("u"),
		a,
		openai.ToolMessage("only one", "c1"),
	}
	out := trimInheritedParentMessages(src, 32)
	if len(out) != 1 {
		t.Fatalf("len=%d want 1 (user only)", len(out))
	}
	if out[0].OfUser == nil {
		t.Fatal("expected user only")
	}
}

func TestTrimInheritedParentMessages_dropsLeadingOrphanTools(t *testing.T) {
	src := []openai.ChatCompletionMessageParamUnion{
		openai.ToolMessage("orphan", "ghost"),
		openai.UserMessage("u"),
		openai.AssistantMessage("a"),
	}
	out := trimInheritedParentMessages(src, 32)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
}

func TestTrimInheritedParentMessages_tailCapThenSanitize(t *testing.T) {
	src := make([]openai.ChatCompletionMessageParamUnion, 0, 40)
	for i := range 40 {
		src = append(src, openai.UserMessage(string(rune('a'+i%26))))
	}
	pending := assistantWithToolCalls("call_x")
	src = append(src, pending)
	out := trimInheritedParentMessages(src, 5)
	// Last 5 would be 4× user + pending assistant; sanitizer drops the incomplete assistant.
	if len(out) != 4 {
		t.Fatalf("len=%d want 4", len(out))
	}
	for _, m := range out {
		if m.OfAssistant != nil && len(m.OfAssistant.ToolCalls) > 0 {
			t.Fatal("tail must not contain unresolved assistant tool_calls")
		}
	}
}

func TestWriteSidechain_flatInstructionRoot(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "workspace")
	instructionRoot := t.TempDir()
	parent := toolctx.New(cwd, nil)
	parent.WorkspaceFlat = true
	parent.InstructionRoot = instructionRoot
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("hi"),
		openai.AssistantMessage("hello"),
	}

	path, err := writeSidechain(parent, "agent1", "run_agent", msgs)
	if err != nil {
		t.Fatal(err)
	}
	if path == "" {
		t.Fatal("expected sidechain path")
	}
	if !strings.Contains(path, filepath.Join(instructionRoot, "sidechain")) {
		t.Fatalf("path = %q, want under %q", path, filepath.Join(instructionRoot, "sidechain"))
	}
	if strings.Contains(path, filepath.Join(cwd, ".oneclaw")) {
		t.Fatalf("path should not use nested .oneclaw under cwd: %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat %q: %v", path, err)
	}
}

func TestHostRunTurn_UsesOverride(t *testing.T) {
	want := errors.New("override-runner")
	h := &Host{
		RunTurn: func(ctx context.Context, cfg loop.Config, in bus.InboundMessage) error {
			return want
		},
	}
	err := h.runTurn(context.Background(), loop.Config{}, bus.InboundMessage{Content: "x"})
	if !errors.Is(err, want) {
		t.Fatalf("runTurn error = %v, want %v", err, want)
	}
}

func TestValidateNestedHost_requiresRunTurn(t *testing.T) {
	c := openai.NewClient()
	h := &Host{
		Client:   &c,
		Registry: tools.NewRegistry(),
	}
	if err := validateNestedHost(h); err == nil {
		t.Fatal("expected error when RunTurn is nil")
	}
}

func TestRunTurn_requiresHostCallback(t *testing.T) {
	h := &Host{}
	err := h.runTurn(context.Background(), loop.Config{}, bus.InboundMessage{})
	if err == nil {
		t.Fatal("expected error")
	}
}
