//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

// E2E-40 write_file 仅依赖 cwd（无 memory 根时路径必须在 cwd 下）
func TestE2E_40_WriteFileUnderCwdOnly(t *testing.T) {
	stub := openaistub.New(t)
	args, _ := json.Marshal(map[string]string{"path": "onlycwd_marker.txt", "content": "ok"})
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("w", "write_file", string(args)),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "wrote"))
	e2eEnvMinimal(t, stub)

	cwd := t.TempDir()
	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "test",
		MaxTokens:   256,
		MaxSteps:    8,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: toolctx.New(cwd, context.Background()),
	}, bus.InboundMessage{Content: "write"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(cwd, "onlycwd_marker.txt"))
	if err != nil || string(b) != "ok" {
		t.Fatalf("file=%v err=%v", b, err)
	}
}

// E2E-41 write_file 到用户级 memory 根（绝对路径，cwd 与 HOME 分离）
func TestE2E_41_WriteFileUnderUserMemoryRoot(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	memFile := filepath.Join(home, ".oneclaw", "memory", "user_mem_e2e.txt")
	args, err := json.Marshal(map[string]string{"path": memFile, "content": "from-home-root"})
	if err != nil {
		t.Fatal(err)
	}
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("w", "write_file", string(args)),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)

	lay := memory.DefaultLayout(cwd, home)
	tctx := toolctx.New(cwd, context.Background())
	tctx.MemoryWriteRoots = lay.WriteRoots()

	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err = loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "test",
		MaxTokens:   256,
		MaxSteps:    8,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: tctx,
	}, bus.InboundMessage{Content: "write to user memory"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(memFile)
	if err != nil || string(b) != "from-home-root" {
		t.Fatalf("read %q err=%v", b, err)
	}
}

// E2E-42 越权路径：绝对路径不在 cwd 也不在 memory 根内
func TestE2E_42_WriteFileRejectedOutsideRoots(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	outside := filepath.Join(t.TempDir(), "forbidden_e2e.txt")
	stub := openaistub.New(t)
	args, _ := json.Marshal(map[string]string{"path": outside, "content": "x"})
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("w", "write_file", string(args)),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "after-deny"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)

	lay := memory.DefaultLayout(cwd, home)
	tctx := toolctx.New(cwd, context.Background())
	tctx.MemoryWriteRoots = lay.WriteRoots()

	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "test",
		MaxTokens:   256,
		MaxSteps:    8,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: tctx,
	}, bus.InboundMessage{Content: "write"})
	if err != nil {
		t.Fatal(err)
	}
	var toolOut string
	for _, m := range msgs {
		if m.OfTool != nil && m.OfTool.Content.OfString.Valid() {
			toolOut = m.OfTool.Content.OfString.Value
			break
		}
	}
	if !strings.Contains(toolOut, "outside") && !strings.Contains(toolOut, "memory roots") && !strings.Contains(toolOut, "working directory") {
		t.Fatalf("expected path error, got %q", toolOut)
	}
	if _, err := os.Stat(outside); err == nil {
		t.Fatal("forbidden file should not exist")
	}
}

// E2E-43 grep 在 .oneclaw/memory 下（memory 根白名单）
func TestE2E_43_GrepUnderProjectMemoryRoot(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	memDir := filepath.Join(cwd, memory.DotDir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(memDir, "needle.md"), []byte("GREP_UNIQUE_E2E_43\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stub := openaistub.New(t)
	gargs, _ := json.Marshal(map[string]string{"pattern": "GREP_UNIQUE_E2E_43", "path": filepath.Join(memory.DotDir, "memory")})
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("g", "grep", string(gargs)),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "found"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)

	lay := memory.DefaultLayout(cwd, home)
	tctx := toolctx.New(cwd, context.Background())
	tctx.MemoryWriteRoots = lay.WriteRoots()

	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "test",
		MaxTokens:   256,
		MaxSteps:    8,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: tctx,
	}, bus.InboundMessage{Content: "search"})
	if err != nil {
		t.Fatal(err)
	}
	var grepOut string
	for _, m := range msgs {
		if m.OfTool != nil && m.OfTool.Content.OfString.Valid() {
			grepOut = m.OfTool.Content.OfString.Value
			break
		}
	}
	if !strings.Contains(grepOut, "GREP_UNIQUE_E2E_43") {
		t.Fatalf("grep output: %q", grepOut)
	}
}
