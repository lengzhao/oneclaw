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
	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

// E2E-111 cron 工具 add 写入 .oneclaw/scheduled_jobs.json
func TestE2E_111_CronToolWritesFile(t *testing.T) {
	cwd := t.TempDir()
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("c1", "cron", `{"action":"add","message":"E2E111_REMINDER","name":"e2e111","schedule":{"every_seconds":7200}}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "done"))
	e2eEnvMinimal(t, stub)

	client := openai.NewClient(stubOpenAIOptions(stub)...)
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
	}, bus.InboundMessage{Content: "schedule a job", ClientID: "cli"})
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(cwd, memory.DotDir, "scheduled_jobs.json")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("scheduled_jobs.json: %v", err)
	}
	var doc struct {
		Jobs []struct {
			Message string `json:"message"`
			Enabled bool   `json:"enabled"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Jobs) != 1 || doc.Jobs[0].Message != "E2E111_REMINDER" || !doc.Jobs[0].Enabled {
		t.Fatalf("unexpected schedule file: %s", string(b))
	}
}

// E2E-112 存在启用的 scheduled job 时 system 含 Scheduled jobs 段
func TestE2E_112_ScheduledJobsBlockInSystemPrompt(t *testing.T) {
	cwd := t.TempDir()
	if _, err := schedule.Add(cwd, "", false, schedule.AddInput{
		Name:    "E2E112",
		Message: "E2E112_MSG",
		Schedule: schedule.ScheduleSpec{
			EverySeconds: 3600,
		},
	}); err != nil {
		t.Fatal(err)
	}

	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub)

	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
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
	if !strings.Contains(sys, "## Scheduled jobs") {
		t.Fatalf("system missing schedule section:\n%s", sys)
	}
	if !strings.Contains(sys, "E2E112_MSG") {
		t.Fatalf("system missing message:\n%s", sys)
	}
}
