//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	lzmodel "github.com/lengzhao/memory/model"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-32 同一会话内 recall 去重：第二轮不再附加已 surface 过的记忆 id
func TestE2E_32_RecallPathDedupSecondTurn(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "t1"))
	stub.Enqueue(openaistub.CompletionStop("", "t2"))
	e2eEnvWithMemory(t, stub)
	s := rtopts.Current()
	s.DisableMemoryExtract = true
	rtopts.Set(&s)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	lay := memory.DefaultLayout(cwd, home)
	token := "recall_dedup_e2e_32 unique content"
	if err := memory.SeedAgentMemoryItem(lay, e.SessionID, lzmodel.NamespaceTypeKnowledge, "dedup", token+"\n"); err != nil {
		t.Fatal(err)
	}

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
	if !strings.Contains(t1, "relevant_memories") || !strings.Contains(t1, token) {
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
	if strings.Contains(t2, "relevant_memories") {
		t.Fatalf("dedup: second request should not re-attach recall block:\n%s", t2)
	}
}

// E2E-33 recall 总字节预算：多条 SQLite 命中时单轮附件体积受 MaxSurfacedRecallBytes 约束
func TestE2E_33_RecallTotalByteBudget(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	keyword := "budcap_e2e_33_kw"
	lay := memory.DefaultLayout(cwd, home)
	for i := 0; i < 6; i++ {
		body := strings.Repeat("x", 3500) + "\n" + keyword + "\n"
		if err := memory.SeedAgentMemoryItem(lay, "", lzmodel.NamespaceTypeKnowledge, fmt.Sprintf("cap%d", i), body); err != nil {
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
