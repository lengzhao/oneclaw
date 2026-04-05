package e2e_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
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
	// 避免 PostTurn 写入的 daily log .md 在第二轮被 recall 命中（与 dedup.md 无关的第二次附件）
	t.Setenv("ONCLAW_DISABLE_MEMORY_EXTRACT", "1")
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, cwd)

	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "recall_dedup_e2e_32 first turn"}); err != nil {
		t.Fatal(err)
	}
	t1 := concatUserText(e.Messages)
	if !strings.Contains(t1, "relevant_memories") || !strings.Contains(t1, onlyPath) {
		t.Fatalf("turn1 missing recall:\n%s", t1)
	}

	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "recall_dedup_e2e_32 second turn"}); err != nil {
		t.Fatal(err)
	}
	full := concatUserText(e.Messages)
	if strings.Count(full, "Attachment: relevant_memories") != 1 {
		t.Fatalf("expected exactly one recall attachment across two turns; got %d",
			strings.Count(full, "Attachment: relevant_memories"))
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
		p := filepath.Join(memDir, fmt.Sprintf("f%d.md", i))
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvWithMemory(t, stub)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, cwd)
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: keyword + " please"}); err != nil {
		t.Fatal(err)
	}
	var recallLen int
	for _, m := range e.Messages {
		if m.OfUser == nil || !m.OfUser.Content.OfString.Valid() {
			continue
		}
		s := m.OfUser.Content.OfString.Value
		if strings.Contains(s, "Attachment: relevant_memories") {
			recallLen = len(s)
			break
		}
	}
	if recallLen <= 0 {
		t.Fatal("expected recall user message")
	}
	if recallLen > memory.MaxSurfacedRecallBytes+64 {
		t.Fatalf("recall message too large: %d > MaxSurfacedRecallBytes+64", recallLen)
	}
}
