package subagent

import (
	"context"
	"strings"

	"github.com/lengzhao/oneclaw/notify"
)

func emitSubagentStart(h *Host, ctx context.Context, kind, agentType, task string, inherit bool, depth int, childRunID, nestedTurnID string) {
	if h == nil || h.Notify == nil {
		return
	}
	ev := notify.NewEvent(notify.EventSubagentStart, "")
	ev.SessionID = h.SessionID
	ev.CorrelationID = h.ParentCorrelationID
	ev.TurnID = nestedTurnID
	ev.RunID = childRunID
	ev.AgentID = strings.TrimSpace(agentType)
	ev.ParentAgentID = strings.TrimSpace(h.ParentAgentID)
	ev.ParentRunID = h.ParentTurnID
	ev.Data = map[string]any{
		"kind":            kind,
		"agent_type":      agentType,
		"task_preview":    notify.Preview(task, notify.DefaultPreviewRunes),
		"inherit_context": inherit,
		"subagent_depth":  depth,
		"child_run_id":    childRunID,
	}
	notify.EmitSafe(h.Notify, ctx, ev)
}

func emitSubagentEnd(h *Host, ctx context.Context, kind, agentType string, depth int, childRunID, nestedTurnID, reply string, runErr error) {
	if h == nil || h.Notify == nil {
		return
	}
	ev := notify.NewEvent(notify.EventSubagentEnd, "")
	if runErr != nil {
		ev.Severity = "error"
	}
	ev.SessionID = h.SessionID
	ev.CorrelationID = h.ParentCorrelationID
	ev.TurnID = nestedTurnID
	ev.RunID = childRunID
	ev.AgentID = strings.TrimSpace(agentType)
	ev.ParentAgentID = strings.TrimSpace(h.ParentAgentID)
	ev.ParentRunID = h.ParentTurnID
	m := map[string]any{
		"kind":           kind,
		"agent_type":     agentType,
		"subagent_depth": depth,
		"child_run_id":   childRunID,
		"ok":             runErr == nil,
		"result_preview": notify.Preview(reply, notify.DefaultPreviewRunes),
	}
	if runErr != nil {
		m["err"] = runErr.Error()
	}
	ev.Data = m
	notify.EmitSafe(h.Notify, ctx, ev)
}
