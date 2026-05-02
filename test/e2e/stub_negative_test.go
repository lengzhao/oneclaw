//go:build e2e

package e2e_test

import (
	"context"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

// E2E-81 空用户输入被拒绝
func TestE2E_81_EmptyInboundRejected(t *testing.T) {
	stub := openaistub.New(t)
	e2eEnvMinimal(t, stub)
	e := newStubEngine(t, stub, t.TempDir())
	err := e.SubmitUser(context.Background(), stubInbound("   "))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("err=%v", err)
	}
}

// E2E-82 模型返回未在 ADK 工具集中注册的工具名时，Eino 在工具节点失败（与旧 loop 将 unknown 写入 tool 行不同）。
func TestE2E_82_UnknownToolName(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("c1", "nonexistent_tool_xyz", `{}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "done"))
	e2eEnvMinimal(t, stub)

	e := newStubEngineWithRegistry(t, stub, t.TempDir(), builtin.DefaultRegistry())
	err := e.SubmitUser(context.Background(), stubInbound("hi"))
	if err == nil {
		t.Fatal("expected error when stub requests undefined tool name")
	}
	if !strings.Contains(err.Error(), "nonexistent_tool_xyz") && !strings.Contains(err.Error(), "not found") {
		t.Fatalf("unexpected error: %v", err)
	}
}
