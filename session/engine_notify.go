package session

import (
	"context"
	"strings"

	"github.com/lengzhao/oneclaw/instructions"
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

func (e *Engine) emitUserInputHook(ctx context.Context, turnID, corrID string, data map[string]any) {
	if e != nil {
		if rel := strings.TrimSpace(e.executionLogRel); rel != "" {
			if data == nil {
				data = map[string]any{}
			}
			data["execution_log"] = rel
		}
	}
	if e.execJournalWanted() {
		rec := map[string]any{"record": "user_input", "turn_id": turnID, "correlation_id": corrID}
		for k, v := range data {
			rec[k] = v
		}
		e.appendExecutionRecord(ctx, rec)
	}
	if !e.hasNotify() {
		return
	}
	ev := notify.NewEvent(notify.EventUserInput, "")
	e.applyNotifyCorrelation(&ev, turnID, corrID)
	ev.Data = data
	notify.EmitSafe(e.Notify, ctx, ev)
}

func (e *Engine) emitTurnStartHook(ctx context.Context, turnID, corrID string, data map[string]any) {
	if e != nil {
		if rel := strings.TrimSpace(e.executionLogRel); rel != "" {
			if data == nil {
				data = map[string]any{}
			}
			data["execution_log"] = rel
		}
	}
	if e.execJournalWanted() {
		rec := map[string]any{"record": "turn_start", "turn_id": turnID, "correlation_id": corrID}
		for k, v := range data {
			rec[k] = v
		}
		e.appendExecutionRecord(ctx, rec)
	}
	if !e.hasNotify() {
		return
	}
	ev := notify.NewEvent(notify.EventTurnStart, "")
	e.applyNotifyCorrelation(&ev, turnID, corrID)
	ev.Data = data
	notify.EmitSafe(e.Notify, ctx, ev)
}

func (e *Engine) emitTurnEndHook(ctx context.Context, turnID, corrID string, ok bool, journal map[string]any) {
	if e.execJournalWanted() {
		rec := map[string]any{"record": "turn_end", "turn_id": turnID, "correlation_id": corrID, "ok": ok}
		for k, v := range journal {
			rec[k] = v
		}
		e.appendExecutionRecord(ctx, rec)
	}
	if !e.hasNotify() {
		return
	}
	ev := notify.NewEvent(notify.EventTurnEnd, "")
	if !ok {
		ev.Severity = "error"
	}
	e.applyNotifyCorrelation(&ev, turnID, corrID)
	lite := map[string]any{"ok": ok}
	if e != nil {
		if rel := strings.TrimSpace(e.executionLogRel); rel != "" {
			lite["execution_log"] = rel
		}
	}
	if !ok {
		if c, ok2 := journal["code"]; ok2 {
			lite["code"] = c
		}
		if m, ok2 := journal["message"]; ok2 {
			lite["message"] = m
		}
		if b, ok2 := journal["truncated_by_max_steps"]; ok2 {
			lite["truncated_by_max_steps"] = b
		}
	} else {
		if tc, ok2 := journal["tool_count"]; ok2 {
			lite["tool_count"] = tc
		}
		if p, ok2 := journal["final_assistant_preview"]; ok2 {
			lite["final_assistant_preview"] = p
		}
		if ls, ok2 := journal["local_slash"]; ok2 {
			lite["local_slash"] = ls
		}
	}
	ev.Data = lite
	notify.EmitSafe(e.Notify, ctx, ev)
}

// emitInstructionContextJournal appends the assembled instruction bundle to the turn execution shard (not a notify event).
func (e *Engine) emitInstructionContextJournal(ctx context.Context, turnID, corrID string, memOK bool, bundle instructions.TurnBundle) {
	if !e.execJournalWanted() {
		return
	}
	data := map[string]any{
		"record":               "instruction_context",
		"turn_id":              turnID,
		"correlation_id":       corrID,
		"memory_enabled":       memOK,
		"agent_md_block":       bundle.AgentMdBlock,
		"agent_md_block_bytes": len(bundle.AgentMdBlock),
	}
	if memOK {
		data["memory_system_prompt_block"] = bundle.SystemSuffix
		data["memory_system_prompt_block_bytes"] = len(bundle.SystemSuffix)
	}
	e.appendExecutionRecord(ctx, data)
}
