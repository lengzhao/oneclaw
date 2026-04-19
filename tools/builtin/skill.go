package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lengzhao/oneclaw/skills"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type invokeSkillInput struct {
	Skill string `json:"skill"`
}

// InvokeSkillTool loads a project or user SKILL.md body (after frontmatter) into the conversation.
type InvokeSkillTool struct{}

func (InvokeSkillTool) Name() string          { return "invoke_skill" }
func (InvokeSkillTool) ConcurrencySafe() bool { return false }

func (InvokeSkillTool) Description() string {
	return `Load the full text of a skill from the user or session skill catalog. Use when the user's task matches a skill listed in the system prompt. Pass the skill directory name (e.g. "pdf"), not a file path.`
}

func (InvokeSkillTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"skill": map[string]any{
			"type":        "string",
			"description": "Skill name (directory name under the skill catalog), same as in the Skills list",
		},
	}, []string{"skill"})
}

func (InvokeSkillTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in invokeSkillInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	name := strings.TrimSpace(in.Skill)
	if name == "" {
		return "", fmt.Errorf("skill name is required")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("user home: %w", err)
	}
	sk, ok := skills.Lookup(tctx.CWD, home, name, tctx.WorkspaceFlat, tctx.InstructionRoot)
	if !ok {
		return "", fmt.Errorf("unknown skill %q (expected a skill directory with SKILL.md in the active skill catalog)", name)
	}
	body, err := sk.PromptBody()
	if err != nil {
		return "", err
	}
	if err := skills.RecordUse(tctx.CWD, sk.Name, tctx.WorkspaceFlat, tctx.InstructionRoot); err != nil {
		slog.Warn("skills.recent_write", "err", err)
	}
	return body, nil
}
