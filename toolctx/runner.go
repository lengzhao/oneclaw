package toolctx

import "context"

// SubagentRunner runs nested query loops for run_agent / fork_context tools (phase C).
type SubagentRunner interface {
	RunAgent(ctx context.Context, parent *Context, agentType, taskPrompt string, inheritContext bool) (string, error)
	RunFork(ctx context.Context, parent *Context, taskPrompt string, maxParentMessages int) (string, error)
}
