//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-32 同一会话内 recall 路径去重：第二轮不再附加已 surface 过的文件
func TestE2E_32_RecallPathDedupSecondTurn(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	memDir := filepath.Join(cwd, memory.DotDir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	onlyPath := filepath.Join(memDir, "dedup.md")
	if err := os.WriteFile(onlyPath, []byte("recall_dedup_e2e_32 unique content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "t1"))
	stub.Enqueue(openaistub.CompletionStop("", "t2"))
	e2eEnvWithMemory(t, stub)
	s := rtopts.Current()
	s.DisableMemoryExtract = true
	rtopts.Set(&s)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)

	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "recall_dedup_e2e_32 first turn"}); err != nil {
		t.Fatal(err)
	}
	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatal("expected first chat request")
	}
	t1, err := openaistub.ChatRequestUserTextConcat(bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(t1, "relevant_memories") || !strings.Contains(t1, onlyPath) {
		t.Fatalf("turn1 missing recall:\n%s", t1)
	}

	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "recall_dedup_e2e_32 second turn"}); err != nil {
		t.Fatal(err)
	}
	bodies = stub.ChatRequestBodies()
	if len(bodies) < 2 {
		t.Fatal("expected second chat request")
	}
	t2, err := openaistub.ChatRequestUserTextConcat(bodies[1])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(t2, onlyPath) {
		t.Fatalf("dedup: second request should not re-surface same path:\n%s", t2)
	}
}

// E2E-33 recall 总字节预算：多文件命中时单轮附件体积受 MaxSurfacedRecallBytes 约束
func TestE2E_33_RecallTotalByteBudget(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	memDir := filepath.Join(cwd, memory.DotDir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	keyword := "budcap_e2e_33_kw"
	for i := 0; i < 6; i++ {
		body := strings.Repeat("x", 3500) + "\n" + keyword + "\n"
		p := filepath.Join(memDir, fmt.Sprintf("budcap_f%d.md", i))
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: keyword + " please"}); err != nil {
		t.Fatal(err)
	}
	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatal("expected chat request")
	}
	recallBlock, ok, err := openaistub.FirstChatUserMessageContaining(bodies[0], "Attachment: relevant_memories")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected recall user message in first request")
	}
	recallLen := len(recallBlock)
	if recallLen > memory.MaxSurfacedRecallBytes+64 {
		t.Fatalf("recall message too large: %d > MaxSurfacedRecallBytes+64", recallLen)
	}
}
