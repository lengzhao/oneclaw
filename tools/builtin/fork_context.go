package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

// ForkContextTool runs a cheap forked loop: same system prompt as the parent plus a trimmed parent tail (phase C).
type ForkContextTool struct{}

func (ForkContextTool) Name() string { return "fork_context" }

func (ForkContextTool) Description() string {
	return `Run auxiliary reasoning with the same system prompt as the main thread and a truncated copy of recent messages. ` +
		`Bash is disabled in this path. The main transcript is unchanged; only the returned text is visible to you.`
}

func (ForkContextTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"prompt": map[string]any{
			"type":        "string",
			"description": "Question or instruction for the forked pass.",
		},
		"max_parent_messages": map[string]any{
			"type":        "integer",
			"description": "Max recent messages to include from the parent thread (default 32).",
		},
	}, []string{"prompt"})
}

func (ForkContextTool) ConcurrencySafe() bool { return false }

func (ForkContextTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	if tctx == nil || tctx.Subagent == nil {
		return "", fmt.Errorf("fork_context: subagent runner not available")
	}
	var args struct {
		Prompt            string `json:"prompt"`
		MaxParentMessages int    `json:"max_parent_messages"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("fork_context: %w", err)
	}
	if strings.TrimSpace(args.Prompt) == "" {
		return "", fmt.Errorf("fork_context: prompt is required")
	}
	return tctx.Subagent.RunFork(ctx, tctx, strings.TrimSpace(args.Prompt), args.MaxParentMessages)
}
