package subagent

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/lengzhao/oneclaw/tools"
)

type runAgentIn struct {
	AgentType string `json:"agent_type" jsonschema:"description=Catalog agent id (agents/*.md stem)"`
	Task      string `json:"task" jsonschema:"description=User message for the sub-agent (isolated context)"`
}

// RegisterRunAgent adds the run_agent meta-tool to r. deps.ParentRegistry is set to r.
// The bound handler uses a shallow snapshot of deps so later mutations do not affect this tool.
func RegisterRunAgent(r *tools.Registry, deps *RunAgentDeps) error {
	if r == nil || deps == nil {
		return fmt.Errorf("subagent: RegisterRunAgent: nil argument")
	}
	snap := *deps
	snap.ParentRegistry = r

	tool, err := utils.InferTool("run_agent", "Delegate a task to another catalog agent with isolated message history and a subset of your tools.",
		func(ctx context.Context, in runAgentIn) (string, error) {
			subType := strings.TrimSpace(in.AgentType)
			if subType == "" {
				return "", fmt.Errorf("agent_type required")
			}
			if snap.Catalog == nil {
				return "", fmt.Errorf("catalog not configured")
			}
			sub := snap.Catalog.Get(subType)
			if sub == nil {
				return "", fmt.Errorf("unknown agent_type %q", subType)
			}
			task := strings.TrimSpace(in.Task)
			if task == "" {
				return "", fmt.Errorf("task required")
			}
			return ExecuteSubAgentTurn(ctx, &snap, sub, task)
		})
	if err != nil {
		return err
	}
	return r.Register(tool)
}
