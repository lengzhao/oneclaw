//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/workspace"
)

// E2E-10 用户级 ~/.oneclaw/AGENT.md 注入
func TestE2E_10_UserAgentMdInjected(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	oneclaw := filepath.Join(home, workspace.DotDir)
	if err := os.MkdirAll(oneclaw, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oneclaw, workspace.AgentInstructionsFile), []byte("E2E10_USER_MARKER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, t.TempDir())
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
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
	if !strings.Contains(reqText, "E2E10_USER_MARKER") {
		t.Fatalf("first request user payload:\n%s", reqText)
	}
}

// E2E-11 项目 `.oneclaw/AGENT.md` 注入（不再使用仓库根 AGENT.md）
func TestE2E_11_ProjectOneclawAgentMd(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	dot := filepath.Join(cwd, workspace.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, workspace.AgentInstructionsFile), []byte("E2E11_PROJECT_MARKER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
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
	if !strings.Contains(reqText, "E2E11_PROJECT_MARKER") {
		t.Fatal(reqText)
	}
}

// E2E-12 仅 .oneclaw/AGENT.md（根目录无 AGENT.md）
func TestE2E_12_DotOneclawAgentMdOnly(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	dot := filepath.Join(cwd, workspace.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, workspace.AgentInstructionsFile), []byte("E2E12_DOTONLY_MARKER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
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
	if !strings.Contains(reqText, "E2E12_DOTONLY_MARKER") {
		t.Fatal(reqText)
	}
}

// E2E-13 .oneclaw/rules/*.md
func TestE2E_13_DotOneclawRules(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	rules := filepath.Join(cwd, workspace.DotDir, "rules")
	if err := os.MkdirAll(rules, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rules, "rule.md"), []byte("E2E13_RULE_MARKER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
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
	if !strings.Contains(reqText, "E2E13_RULE_MARKER") {
		t.Fatal(reqText)
	}
}

// E2E-14 向上遍历：父目录与 cwd 均有 AGENT.md，子目录标记应出现在父标记之后（后加载更具体）
func TestE2E_14_WalkUpOrderChildAfterParent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	parent := t.TempDir()
	child := filepath.Join(parent, "child")
	if err := os.MkdirAll(child, 0o755); err != nil {
		t.Fatal(err)
	}
	pDot := filepath.Join(parent, workspace.DotDir)
	cDot := filepath.Join(child, workspace.DotDir)
	if err := os.MkdirAll(pDot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cDot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pDot, workspace.AgentInstructionsFile), []byte("E2E14_PARENT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cDot, workspace.AgentInstructionsFile), []byte("E2E14_CHILD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, child)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
		t.Fatal(err)
	}
	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatal("expected chat request")
	}
	text, err := openaistub.ChatRequestUserTextConcat(bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	iP := strings.Index(text, "E2E14_PARENT")
	iC := strings.Index(text, "E2E14_CHILD")
	if iP < 0 || iC < 0 {
		t.Fatalf("markers missing:\n%s", text)
	}
	if !(iP < iC) {
		t.Fatalf("want parent before child in concatenation, got parent@%d child@%d", iP, iC)
	}
}

// E2E-15 e2eEnvMinimal（DisableMemory）时不注入 AGENT 内容
func TestE2E_15_MemoryDisabledNoAgentInject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cwd := t.TempDir()
	dot := filepath.Join(cwd, workspace.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dot, workspace.AgentInstructionsFile), []byte("E2E15_SHOULD_NOT_APPEAR\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(concatUserText(e.Messages), "E2E15_SHOULD_NOT_APPEAR") {
		t.Fatal("marker should not be injected")
	}
}

// E2E-16 HOME 未定义（空）：不崩溃，memory 注入跳过
func TestE2E_16_NoHomeDegradesGracefully(t *testing.T) {
	t.Setenv("HOME", "")
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, "")
	e := newStubEngine(t, stub, t.TempDir())
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
		t.Fatal(err)
	}
}
