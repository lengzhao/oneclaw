package sinks

import (
	"context"

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

// VisibleAuditSink writes a full user-visible transcript snapshot on each turn_complete.
type VisibleAuditSink struct {
	opts   Options
	getter func() []openai.ChatCompletionMessageParamUnion
}

// NewVisibleAuditSink returns a notify.Sink; getter should return the current session transcript
// (typically Engine.Transcript); messages are reduced with loop.ToUserVisibleMessages before export.
func NewVisibleAuditSink(o Options, getter func() []openai.ChatCompletionMessageParamUnion) notify.Sink {
	if getter == nil {
		getter = func() []openai.ChatCompletionMessageParamUnion { return nil }
	}
	return &VisibleAuditSink{opts: o, getter: getter}
}

func (s *VisibleAuditSink) Emit(ctx context.Context, ev notify.Event) error {
	if ev.Event != notify.EventTurnComplete {
		return nil
	}
	msgs := loop.ToUserVisibleMessages(s.getter())
	transcript := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		switch {
		case m.OfUser != nil:
			t := loop.UserMessageText(m)
			if t == "" {
				continue
			}
			transcript = append(transcript, map[string]string{"role": "user", "content": t})
		case m.OfAssistant != nil:
			t := loop.AssistantParamText(m)
			if t == "" {
				continue
			}
			transcript = append(transcript, map[string]string{"role": "assistant", "content": t})
		default:
			continue
		}
	}
	rec := map[string]any{
		"kind":             "audit_visible",
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
		"transcript":       transcript,
		"turn_data":        ev.Data,
	}
	return appendJSONLRecord(s.opts.CWD, s.opts.AuditSessionID, s.opts.Segment(), "visible", wallTimeFromEventTS(ev.TS), rec)
}
