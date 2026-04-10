package sinks

import (
	"context"
	"fmt"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/notify"
	"github.com/openai/openai-go"
)

// LLMAuditSink writes model_step_start, turn_first_model_request, and model_step_end to llm/*.jsonl.
type LLMAuditSink struct {
	opts Options
}

// NewLLMAuditSink returns a notify.Sink for per-step LLM audit lines.
func NewLLMAuditSink(o Options) notify.Sink {
	return &LLMAuditSink{opts: o}
}

func (s *LLMAuditSink) Emit(ctx context.Context, ev notify.Event) error {
	switch ev.Event {
	case notify.EventModelStepStart, notify.EventModelStepEnd, notify.EventTurnFirstModelRequest:
	default:
		return nil
	}
	rec := map[string]any{
		"kind":             "audit_llm",
		"notify_event":     ev.Event,
		"ts":               ev.TS,
		"schema_version":   ev.SchemaVersion,
		"session_id":       ev.SessionID,
		"agent_id":         ev.AgentID,
		"correlation_id":   ev.CorrelationID,
		"turn_id":          ev.TurnID,
		"run_id":           ev.RunID,
		"parent_agent_id":  ev.ParentAgentID,
		"parent_run_id":    ev.ParentRunID,
		"severity":         ev.Severity,
		"data":             ev.Data,
	}
	return appendJSONLRecord(s.opts.CWD, s.opts.AuditSessionID, s.opts.Segment(), "llm", wallTimeFromEventTS(ev.TS), rec)
}

// OrchestrationAuditSink writes inbound, tools, subagent, and turn boundary events.
type OrchestrationAuditSink struct {
	opts Options
}

// NewOrchestrationAuditSink returns a notify.Sink for orchestration / tool-chain audit lines.
func NewOrchestrationAuditSink(o Options) notify.Sink {
	return &OrchestrationAuditSink{opts: o}
}

func orchestrationEvent(ev string) bool {
	switch ev {
	case notify.EventInboundReceived,
		notify.EventMemoryTurnContext,
		notify.EventAgentTurnStart,
		notify.EventToolCallStart,
		notify.EventToolCallEnd,
		notify.EventSubagentStart,
		notify.EventSubagentEnd,
		notify.EventTurnComplete,
		notify.EventTurnError:
		return true
	default:
		return false
	}
}

func (s *OrchestrationAuditSink) Emit(ctx context.Context, ev notify.Event) error {
	if !orchestrationEvent(ev.Event) {
		return nil
	}
	rec := map[string]any{
		"kind":             "audit_orchestration",
		"notify_event":     ev.Event,
		"ts":               ev.TS,
		"schema_version":   ev.SchemaVersion,
		"session_id":       ev.SessionID,
		"agent_id":         ev.AgentID,
		"correlation_id":   ev.CorrelationID,
		"turn_id":          ev.TurnID,
		"run_id":           ev.RunID,
		"parent_agent_id":  ev.ParentAgentID,
		"parent_run_id":    ev.ParentRunID,
		"severity":         ev.Severity,
		"data":             ev.Data,
	}
	return appendJSONLRecord(s.opts.CWD, s.opts.AuditSessionID, s.opts.Segment(), "orchestration", wallTimeFromEventTS(ev.TS), rec)
}

// VisibleAuditSink writes one jsonl line per turn_complete: only this turn's user-visible messages
// (same {role, content} shape as transcript.json), not a full session snapshot.
type VisibleAuditSink struct {
	opts   Options
	getter func() []openai.ChatCompletionMessageParamUnion
}

// NewVisibleAuditSink returns a notify.Sink; getter should return the current session transcript
// (typically Engine.Transcript). Used only when EventTurnComplete.Data["messages"] is missing
// (fallback: last two visible rows at most).
func NewVisibleAuditSink(o Options, getter func() []openai.ChatCompletionMessageParamUnion) notify.Sink {
	if getter == nil {
		getter = func() []openai.ChatCompletionMessageParamUnion { return nil }
	}
	return &VisibleAuditSink{opts: o, getter: getter}
}

func visibleMessagesFromTurnData(data map[string]any) ([]map[string]string, error) {
	if data == nil {
		return nil, nil
	}
	raw, ok := data["messages"]
	if !ok || raw == nil {
		return nil, nil
	}
	switch v := raw.(type) {
	case []map[string]string:
		return v, nil
	case []any:
		out := make([]map[string]string, 0, len(v))
		for i, it := range v {
			m, ok := it.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("messages[%d]: want object", i)
			}
			role, _ := m["role"].(string)
			content, _ := m["content"].(string)
			out = append(out, map[string]string{"role": role, "content": content})
		}
		return out, nil
	default:
		return nil, fmt.Errorf("messages: unsupported type %T", raw)
	}
}

func (s *VisibleAuditSink) Emit(ctx context.Context, ev notify.Event) error {
	if ev.Event != notify.EventTurnComplete {
		return nil
	}
	messages, err := visibleMessagesFromTurnData(ev.Data)
	if err != nil {
		return err
	}
	if len(messages) == 0 && s.getter != nil {
		all := loop.ToUserVisibleMessages(s.getter())
		if len(all) > 0 {
			start := len(all) - 2
			if start < 0 {
				start = 0
			}
			messages = loop.ChatTurnRecords(all[start:])
		}
	}
	rec := map[string]any{
		"kind":            "audit_visible",
		"notify_event":    ev.Event,
		"ts":              ev.TS,
		"schema_version":  ev.SchemaVersion,
		"session_id":      ev.SessionID,
		"agent_id":        ev.AgentID,
		"correlation_id":  ev.CorrelationID,
		"turn_id":         ev.TurnID,
		"run_id":          ev.RunID,
		"parent_agent_id": ev.ParentAgentID,
		"parent_run_id":   ev.ParentRunID,
		"messages":        messages,
	}
	if ev.Data != nil {
		if v, ok := ev.Data["tool_count"]; ok {
			rec["tool_count"] = v
		}
		if v, ok := ev.Data["final_assistant_preview"]; ok {
			rec["final_assistant_preview"] = v
		}
		if v, ok := ev.Data["truncated_by_max_steps"]; ok {
			rec["truncated_by_max_steps"] = v
		}
		if v, ok := ev.Data["local_slash"]; ok {
			rec["local_slash"] = v
		}
	}
	return appendJSONLRecord(s.opts.CWD, s.opts.AuditSessionID, s.opts.Segment(), "visible", wallTimeFromEventTS(ev.TS), rec)
}
