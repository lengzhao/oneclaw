package session

import (
	"context"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/notify"
)

// RegisterNotify appends non-nil sinks to the engine's default notify.Multi (same as e.Notify.Register).
func (e *Engine) RegisterNotify(sinks ...notify.Sink) {
	if e == nil {
		return
	}
	e.Notify.Register(sinks...)
}

func (e *Engine) hasNotify() bool {
	return e != nil && len(e.Notify) > 0
}

// emitMemoryTurnContextNotify records recall / agent-md / memory system block after budget apply, before the model loop.
func (e *Engine) emitMemoryTurnContextNotify(ctx context.Context, turnID, corrID string, memOK bool, bundle memory.TurnBundle) {
	if !e.hasNotify() {
		return
	}
	ev := notify.NewEvent(notify.EventMemoryTurnContext, "")
	e.applyNotifyCorrelation(&ev, turnID, corrID)
	data := map[string]any{
		"memory_enabled":       memOK,
		"recall_block":         bundle.RecallBlock,
		"agent_md_block":       bundle.AgentMdBlock,
		"recall_block_bytes":   len(bundle.RecallBlock),
		"agent_md_block_bytes": len(bundle.AgentMdBlock),
	}
	if memOK {
		data["memory_system_prompt_block"] = bundle.SystemSuffix
		data["memory_system_prompt_block_bytes"] = len(bundle.SystemSuffix)
	}
	ev.Data = data
	notify.EmitSafe(e.Notify, ctx, ev)
}
