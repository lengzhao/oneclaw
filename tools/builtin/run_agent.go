package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

// RunAgentTool delegates to a named sub-agent with an isolated transcript (phase C).
type RunAgentTool struct{}

func (RunAgentTool) Name() string { return "run_agent" }

func (RunAgentTool) Description() string {
	return subagent.RunAgentToolDescriptionBase
}

func (RunAgentTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"agent_type": map[string]any{
			"type":        "string",
			"description": "Agent role name, e.g. explore or general-purpose.",
		},
		"prompt": map[string]any{
			"type":        "string",
			"description": "Task instructions for the sub-agent.",
		},
		"inherit_context": map[string]any{
			"type":        "boolean",
			"description": "If true, prepend the last portion of the parent conversation as background.",
		},
	}, []string{"agent_type", "prompt"})
}

func (RunAgentTool) ConcurrencySafe() bool { return false }

func (RunAgentTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	if tctx == nil || tctx.Subagent == nil {
		return "", fmt.Errorf("run_agent: subagent runner not available")
	}
	var args struct {
		AgentType      string `json:"agent_type"`
		Prompt         string `json:"prompt"`
		InheritContext bool   `json:"inherit_context"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("run_agent: %w", err)
	}
	if strings.TrimSpace(args.AgentType) == "" || strings.TrimSpace(args.Prompt) == "" {
		return "", fmt.Errorf("run_agent: agent_type and prompt are required")
	}
	return tctx.Subagent.RunAgent(ctx, tctx, strings.TrimSpace(args.AgentType), strings.TrimSpace(args.Prompt), args.InheritContext)
}
