//go:build e2e

// Skills：系统提示索引 + invoke_skill 工具闭环（用例编号见 CASES.md）。
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
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

func writeSkill(t *testing.T, cwd, name, description, body string) {
	t.Helper()
	dir := filepath.Join(cwd, memory.DotDir, "skills", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\ndescription: " + description + "\n---\n\n" + body + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// E2E-105 存在 .oneclaw/skills 时，首轮 chat 请求的 system 含 Skills 索引与技能名。
func TestE2E_105_SkillsIndexInSystemPrompt(t *testing.T) {
	cwd := t.TempDir()
	writeSkill(t, cwd, "e2e-demo", "E2E105_SKILL_DESC", "body ignored for index")

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
	if !strings.Contains(sys, "## Skills") {
		t.Fatalf("system missing ## Skills:\n%s", sys)
	}
	if !strings.Contains(sys, "e2e-demo") {
		t.Fatalf("system missing skill name:\n%s", sys)
	}
	if !strings.Contains(sys, "E2E105_SKILL_DESC") {
		t.Fatalf("system missing description:\n%s", sys)
	}
}

// E2E-106 invoke_skill 返回 SKILL 正文并写入 skills-recent.json。
func TestE2E_106_InvokeSkillToolAndRecentFile(t *testing.T) {
	cwd := t.TempDir()
	writeSkill(t, cwd, "e2e-invoke", "invoke me", "E2E106_BODY_MARKER")

	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("call_sk", "invoke_skill", `{"skill":"e2e-invoke"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "after skill"))
	e2eEnvMinimal(t, stub)

	client := openai.NewClient(stubOpenAIOptions(stub)...)
	msgs := []openai.ChatCompletionMessageParamUnion{}
	err := loop.RunTurn(context.Background(), loop.Config{
		Client:      &client,
		Model:       "gpt-4o",
		System:      "You may use tools.",
		MaxTokens:   512,
		MaxSteps:    8,
		Messages:    &msgs,
		Registry:    builtin.DefaultRegistry(),
		ToolContext: toolctx.New(cwd, context.Background()),
	}, bus.InboundMessage{Content: "load the skill"})
	if err != nil {
		t.Fatal(err)
	}

	var toolOut string
	for _, m := range msgs {
		if m.OfTool == nil {
			continue
		}
		if m.OfTool.Content.OfString.Valid() {
			toolOut = m.OfTool.Content.OfString.Value
			break
		}
	}
	if !strings.Contains(toolOut, "E2E106_BODY_MARKER") {
		t.Fatalf("tool output missing body marker:\n%s", toolOut)
	}
	if !strings.Contains(toolOut, "Base directory for this skill:") {
		t.Fatalf("tool output missing base dir line:\n%s", toolOut)
	}

	recentPath := filepath.Join(cwd, memory.DotDir, "skills-recent.json")
	b, err := os.ReadFile(recentPath)
	if err != nil {
		t.Fatalf("skills-recent.json: %v", err)
	}
	var doc struct {
		Entries []struct {
			Name string `json:"name"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) < 1 || doc.Entries[0].Name != "e2e-invoke" {
		t.Fatalf("recent entries: %s", string(b))
	}
}

// E2E-107 rtopts.DisableSkills 时 system 不出现 ## Skills（磁盘上仍有 skill）。
func TestE2E_107_SkillsDisabledNoSystemSection(t *testing.T) {
	cwd := t.TempDir()
	writeSkill(t, cwd, "hidden", "should not appear in system", "x")

	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "ok"))
	e2eEnvMinimal(t, stub)
	s := rtopts.Current()
	s.DisableSkills = true
	rtopts.Set(&s)

	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "ping"}); err != nil {
		t.Fatal(err)
	}

	bodies := stub.ChatRequestBodies()
	sys, err := openaistub.ChatRequestSystemTextConcat(bodies[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(sys, "## Skills") {
		t.Fatalf("expected no ## Skills when disabled, got:\n%s", sys)
	}
}
