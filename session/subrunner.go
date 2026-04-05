package session

import (
	"context"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/toolctx"
)

// subRunner implements toolctx.SubagentRunner for a single user turn.
type subRunner struct {
	eng        *Engine
	turnSystem string
	catalog    *subagent.Catalog
	bg         budget.Global
}

func (r *subRunner) host() *subagent.Host {
	if r == nil || r.eng == nil {
		return nil
	}
	return &subagent.Host{
		Client:               &r.eng.Client,
		Model:                r.eng.Model,
		MaxTokens:            r.eng.MaxTokens,
		MaxSteps:             r.eng.MaxSteps,
		Registry:             r.eng.Registry,
		CanUseTool:           r.eng.CanUseTool,
		CWD:                  r.eng.CWD,
		SessionID:            r.eng.SessionID,
		Catalog:              r.catalog,
		ParentSystem:         r.turnSystem,
		ParentMessages:       &r.eng.Messages,
		MaxInheritedMessages: r.bg.InheritedMessageCap(),
		HistoryBudget:        r.bg,
		ChatTransport:        r.eng.ChatTransport,
	}
}

func (r *subRunner) RunAgent(ctx context.Context, parent *toolctx.Context, agentType, taskPrompt string, inheritContext bool) (string, error) {
	return subagent.RunAgent(ctx, r.host(), parent, agentType, taskPrompt, inheritContext)
}

func (r *subRunner) RunFork(ctx context.Context, parent *toolctx.Context, taskPrompt string, maxParentMessages int) (string, error) {
	return subagent.RunFork(ctx, r.host(), parent, taskPrompt, maxParentMessages)
}
