//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/lengzhao/oneclaw/workspace"
	"github.com/openai/openai-go"
)

// e2eLoadStubEinoRuntimeResolved writes ~/.oneclaw/config.yaml with stub OpenAI URLs (kernel is always eino).
func e2eLoadStubEinoRuntimeResolved(t *testing.T, stub *openaistub.Server) (home string, resolved *config.Resolved) {
	t.Helper()
	home = t.TempDir()
	t.Setenv("HOME", home)
	ud := filepath.Join(home, ".oneclaw")
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgYAML := `
openai:
  api_key: "sk-test-stub"
  base_url: "` + stub.BaseURL() + `"
model: gpt-4o
`
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(cfgYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := config.Load(config.LoadOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	r.PushRuntime()
	return home, r
}

// Stub E2E：MainEngineFactory + Eino；SubmitUser 走 ADK，HTTP 打到 openaistub。
func TestE2E_StubEinoRuntime_SubmitUser(t *testing.T) {
	stub := openaistub.New(t)
	for range 16 {
		stub.Enqueue(openaistub.CompletionStop("", "hello from eino stub"))
	}
	e2eEnvMinimal(t, stub)

	_, resolved := e2eLoadStubEinoRuntimeResolved(t, stub)

	var mu sync.Mutex
	var texts []string
	br, cleanup := e2eStartNoopBridge(t, []string{"cli"}, func(msg *bus.OutboundMessage) {
		mu.Lock()
		defer mu.Unlock()
		if msg != nil && strings.TrimSpace(msg.Text) != "" {
			texts = append(texts, msg.Text)
		}
	})
	defer cleanup()

	f := session.MainEngineFactory(session.MainEngineFactoryDeps{
		Resolved: resolved,
		Registry: builtin.DefaultRegistry(),
		Client:   openai.NewClient(stubOpenAIOptions(stub)...),
		Model:    resolved.ChatModel(),
		Bridge:   br,
	})

	in := bus.InboundMessage{ClientID: "cli", SessionID: "C1", Content: "hello eino e2e"}
	eng, err := f(session.SessionHandle{Source: in.ClientID, SessionKey: session.InboundSessionKey(in)})
	if err != nil {
		t.Fatal(err)
	}
	if eng.TurnRunner == nil || eng.TurnRunner.Name() != "eino" {
		t.Fatalf("expected eino runner, got %#v", eng.TurnRunner)
	}

	if err := eng.SubmitUser(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	e2eWaitMinChatRequests(t, stub, 1, 2*time.Second)

	e2eWaitOutboundDispatch(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(texts) >= 1
	})

	mu.Lock()
	defer mu.Unlock()
	var saw bool
	for _, s := range texts {
		if strings.Contains(s, "hello from eino stub") {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatalf("expected stub reply in outbound texts: %v", texts)
	}
}

// Stub E2E：Eino ADK 多轮 — CompletionToolCalls(read_file) → 工具执行 → CompletionStop（对齐 loop E2E-03）。
func TestE2E_StubEinoRuntime_ToolReadFile(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("call_1", "read_file", `{"path":"note.txt"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "read ok"))
	e2eEnvMinimal(t, stub)

	home, resolved := e2eLoadStubEinoRuntimeResolved(t, stub)
	ws := filepath.Join(home, ".oneclaw", "workspace")
	if err := os.MkdirAll(ws, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "note.txt"), []byte("e2e secret"), 0o644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var texts []string
	br, cleanup := e2eStartNoopBridge(t, []string{"cli"}, func(msg *bus.OutboundMessage) {
		mu.Lock()
		defer mu.Unlock()
		if msg != nil && strings.TrimSpace(msg.Text) != "" {
			texts = append(texts, msg.Text)
		}
	})
	defer cleanup()

	f := session.MainEngineFactory(session.MainEngineFactoryDeps{
		Resolved: resolved,
		Registry: builtin.DefaultRegistry(),
		Client:   openai.NewClient(stubOpenAIOptions(stub)...),
		Model:    resolved.ChatModel(),
		Bridge:   br,
	})

	in := bus.InboundMessage{ClientID: "cli", SessionID: "C1", Content: "read note.txt"}
	eng, err := f(session.SessionHandle{Source: in.ClientID, SessionKey: session.InboundSessionKey(in)})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.SubmitUser(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	e2eWaitMinChatRequests(t, stub, 2, 3*time.Second)

	e2eWaitOutboundDispatch(t, func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(texts) >= 1
	})

	mu.Lock()
	defer mu.Unlock()
	var saw bool
	for _, s := range texts {
		if strings.Contains(s, "read ok") {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatalf("expected final assistant text in outbound: %v", texts)
	}
}

// Stub E2E：run_agent 嵌套（对齐 E2E-90）；主子循环均经 TurnRunner=Eino → openaistub。
func TestE2E_StubEinoRuntime_RunAgentNested(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("c1", "run_agent", `{"agent_type":"explore","prompt":"scan repo"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "sub finished"))
	stub.Enqueue(openaistub.CompletionStop("", "parent done"))
	e2eEnvMinimal(t, stub)

	_, resolved := e2eLoadStubEinoRuntimeResolved(t, stub)

	br, cleanup := e2eStartNoopBridge(t, []string{"cli"}, nil)
	defer cleanup()

	f := session.MainEngineFactory(session.MainEngineFactoryDeps{
		Resolved: resolved,
		Registry: builtin.DefaultRegistry(),
		Client:   openai.NewClient(stubOpenAIOptions(stub)...),
		Model:    resolved.ChatModel(),
		Bridge:   br,
	})

	in := bus.InboundMessage{ClientID: "cli", SessionID: "C1", Content: "go"}
	eng, err := f(session.SessionHandle{Source: in.ClientID, SessionKey: session.InboundSessionKey(in)})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.SubmitUser(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	e2eWaitMinChatRequests(t, stub, 3, 5*time.Second)

	if len(eng.Messages) == 0 {
		t.Fatal("empty visible messages after SubmitUser")
	}
	last := eng.Messages[len(eng.Messages)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "parent done" {
		t.Fatalf("expected parent final assistant parent done, got %#v", last)
	}

	sideDir := workspace.JoinSessionWorkspaceWithInstruction(eng.CWD, eng.InstructionRoot, eng.WorkspaceFlat, "sidechain")
	entries, err := os.ReadDir(sideDir)
	if err != nil || len(entries) == 0 {
		t.Fatalf("expected sidechain files under %s: %v entries=%v", sideDir, err, entries)
	}
}

// Stub E2E：fork_context（对齐 E2E-91）。
func TestE2E_StubEinoRuntime_ForkContext(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("f1", "fork_context", `{"prompt":"summarize briefly"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "fork answer"))
	stub.Enqueue(openaistub.CompletionStop("", "ack"))
	e2eEnvMinimal(t, stub)

	_, resolved := e2eLoadStubEinoRuntimeResolved(t, stub)

	br, cleanup := e2eStartNoopBridge(t, []string{"cli"}, nil)
	defer cleanup()

	f := session.MainEngineFactory(session.MainEngineFactoryDeps{
		Resolved: resolved,
		Registry: builtin.DefaultRegistry(),
		Client:   openai.NewClient(stubOpenAIOptions(stub)...),
		Model:    resolved.ChatModel(),
		Bridge:   br,
	})

	in := bus.InboundMessage{ClientID: "cli", SessionID: "C1", Content: "hi"}
	eng, err := f(session.SessionHandle{Source: in.ClientID, SessionKey: session.InboundSessionKey(in)})
	if err != nil {
		t.Fatal(err)
	}
	if err := eng.SubmitUser(context.Background(), in); err != nil {
		t.Fatal(err)
	}

	e2eWaitMinChatRequests(t, stub, 3, 5*time.Second)

	last := eng.Messages[len(eng.Messages)-1]
	if last.OfAssistant == nil || !last.OfAssistant.Content.OfString.Valid() ||
		last.OfAssistant.Content.OfString.Value != "ack" {
		t.Fatalf("expected final ack, got %#v", last)
	}
}
