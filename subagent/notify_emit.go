package subagent

import (
	"context"
	"strings"

	"github.com/lengzhao/oneclaw/notify"
)

func emitSubagentStart(h *Host, ctx context.Context, kind, agentType, task string, inherit bool, depth int, childRunID, nestedTurnID string) {
	if h == nil {
		return
	}
	rec := map[string]any{
		"record":           "subagent_start",
		"session_id":       h.SessionID,
		"correlation_id":   h.ParentCorrelationID,
		"turn_id":          nestedTurnID,
		"run_id":           childRunID,
		"agent_id":         strings.TrimSpace(agentType),
		"parent_agent_id":  strings.TrimSpace(h.ParentAgentID),
		"parent_run_id":    h.ParentTurnID,
		"kind":             kind,
		"agent_type":       agentType,
		"task_preview":     notify.Preview(task, notify.DefaultPreviewRunes),
		"inherit_context":  inherit,
		"subagent_depth":   depth,
		"child_run_id":     childRunID,
	}
	if h.AppendExec != nil {
		h.AppendExec(ctx, rec)
	}
}

func emitSubagentEnd(h *Host, ctx context.Context, kind, agentType string, depth int, childRunID, nestedTurnID, reply string, runErr error) {
	if h == nil {
		return
	}
	rec := map[string]any{
		"record":           "subagent_end",
		"session_id":       h.SessionID,
		"correlation_id":   h.ParentCorrelationID,
		"turn_id":          nestedTurnID,
		"run_id":           childRunID,
		"agent_id":         strings.TrimSpace(agentType),
		"parent_agent_id":  strings.TrimSpace(h.ParentAgentID),
		"parent_run_id":    h.ParentTurnID,
		"kind":             kind,
		"agent_type":       agentType,
		"subagent_depth":   depth,
		"child_run_id":     childRunID,
		"ok":               runErr == nil,
		"result_preview":   notify.Preview(reply, notify.DefaultPreviewRunes),
	}
	if runErr != nil {
		rec["err"] = runErr.Error()
	}
	if h.AppendExec != nil {
		h.AppendExec(ctx, rec)
	}
}
