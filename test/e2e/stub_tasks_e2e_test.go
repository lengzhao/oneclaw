// 任务状态工具：落盘 + 系统提示（用例编号见 CASES.md）。
package e2e_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/tasks"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-108 存在 tasks.json 时 system 含 Task list 与任务摘要。
func TestE2E_108_TasksBlockInSystemPrompt(t *testing.T) {
	cwd := t.TempDir()
	if _, err := tasks.Create(cwd, false, []tasks.CreateInput{{Subject: "E2E108_SUBJECT", Status: "in_progress"}}); err != nil {
		t.Fatal(err)
	}

	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub)

	e := newStubEngine(t, cwd)
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "ping"}); err != nil {
		t.Fatal(err)
	}

	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatal("no chat request captured")
	}
	sys, err := openaistub.ChatRequestSystemTextConcat(bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sys, "## Task list") {
		t.Fatalf("system missing task section:\n%s", sys)
	}
	if !strings.Contains(sys, "E2E108_SUBJECT") {
		t.Fatalf("system missing subject:\n%s", sys)
	}
}

// E2E-109 task_create / task_update 落盘；ONCLAW_DISABLE_TASKS=1 时 system 无 Task list。
func TestE2E_109_TaskToolsWriteFileAndDisableHidesBlock(t *testing.T) {
	cwd := t.TempDir()
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("c1", "task_create", `{"replace":false,"tasks":[{"subject":"E2E109_A","status":"pending"}]}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "done"))
	e2eEnvMinimal(t, stub)

	client := openai.NewClient()
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "Use tools.",
		MaxTokens:   512,
		MaxSteps:    8,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: toolctx.New(cwd, context.Background()),
	}, routing.Inbound{Text: "create task"})
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(cwd, memory.DotDir, "tasks.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("tasks.json: %v", err)
	}
	var doc struct {
		Items []struct {
			ID      string `json:"id"`
			Subject string `json:"subject"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Items) != 1 || doc.Items[0].Subject != "E2E109_A" {
		t.Fatalf("unexpected tasks file: %s", string(b))
	}
	id := doc.Items[0].ID
	if id == "" {
		t.Fatal("empty task id")
	}

	stub2 := openaistub.New(t)
	stub2.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("u1", "task_update", `{"task_id":"`+id+`","status":"completed"}`),
	}))
	stub2.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub2)
	client2 := openai.NewClient()
	msgs2 := []openai.ChatCompletionMessageParamUnion{}
	err = loop.RunTurn(context.Background(), loop.Config{
		Client:      &client2,
		Model:       "gpt-4o",
		System:      "Use tools.",
		MaxTokens:   512,
		MaxSteps:    8,
		Messages:    &msgs2,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: toolctx.New(cwd, context.Background()),
	}, routing.Inbound{Text: "complete it"})
	if err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if !strings.Contains(string(b), "completed") {
		t.Fatalf("expected completed status in file: %s", string(b))
	}

	t.Setenv("ONCLAW_DISABLE_TASKS", "1")
	stub3 := openaistub.New(t)
	stub3.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub3)
	e := newStubEngine(t, cwd)
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "ping"}); err != nil {
		t.Fatal(err)
	}
	bodies := stub3.ChatRequestBodies()
	sys, err := openaistub.ChatRequestSystemTextConcat(bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sys, "## Task list") {
		t.Fatalf("expected no task section when disabled, got:\n%s", sys)
	}
}
