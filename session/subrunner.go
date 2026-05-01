package session

import (
	"context"
	"strings"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/toolctx"
)

// subRunner implements toolctx.SubagentRunner for a single user turn.
type subRunner struct {
	eng                 *Engine
	turnSystem          string
	catalog             *subagent.Catalog
	bg                  budget.Global
	parentTurnID        string
	parentCorrelationID string
}

func parentNotifyAgentID(parent *toolctx.Context) string {
	if parent == nil {
		return DefaultRootAgentID
	}
	if s := strings.TrimSpace(parent.AgentID); s != "" {
		return s
	}
	return DefaultRootAgentID
}

func (r *subRunner) host(parent *toolctx.Context) *subagent.Host {
	if r == nil || r.eng == nil {
		return nil
	}
	h := &subagent.Host{
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
	if r.eng.execJournalWanted() {
		h.AppendExec = r.eng.appendExecutionRecord
	}
	if len(r.eng.Notify) > 0 || r.eng.execJournalWanted() {
		h.ParentAgentID = parentNotifyAgentID(parent)
		h.ParentTurnID = r.parentTurnID
		h.ParentCorrelationID = r.parentCorrelationID
	}
	if len(r.eng.Notify) > 0 {
		h.Notify = r.eng.Notify
	}
	if r.eng.wantsLifecycle() {
		h.OnNestedLifecycle = func(ctx context.Context, childTurnID, childRunID, nestedAgentID string, depth int) *loop.LifecycleCallbacks {
			return r.eng.nestedLoopLifecycle(r.parentTurnID, r.parentCorrelationID, childTurnID, childRunID, nestedAgentID, strings.TrimSpace(h.ParentAgentID), depth)
		}
	}
	return h
}

func (r *subRunner) RunAgent(ctx context.Context, parent *toolctx.Context, agentType, taskPrompt string, inheritContext bool) (string, error) {
	return subagent.RunAgent(ctx, r.host(parent), parent, agentType, taskPrompt, inheritContext)
}

func (r *subRunner) RunFork(ctx context.Context, parent *toolctx.Context, taskPrompt string, maxParentMessages int) (string, error) {
	return subagent.RunFork(ctx, r.host(parent), parent, taskPrompt, maxParentMessages)
}
