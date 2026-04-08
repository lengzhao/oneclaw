package session

import (
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/notify/sinks"
	"github.com/openai/openai-go"
)

// RegisterAuditSinks registers JSONL audit sinks under
// .oneclaw/audit/<agent_segment>/{llm,orchestration,visible}/...
// Pass true per path to enable that sink; all false is a no-op.
// Agent segment is derived from Engine.RootAgentID (see sinks.SanitizeAgentSegment).
func (e *Engine) RegisterAuditSinks(llm, orchestration, visible bool) {
	if e == nil || (!llm && !orchestration && !visible) {
		return
	}
	o := sinks.Options{CWD: e.CWD, AgentID: e.EffectiveRootAgentID()}
	if llm {
		e.RegisterNotify(sinks.NewLLMAuditSink(o))
	}
	if orchestration {
		e.RegisterNotify(sinks.NewOrchestrationAuditSink(o))
	}
	if visible {
		e.RegisterNotify(sinks.NewVisibleAuditSink(o, func() []openai.ChatCompletionMessageParamUnion {
			if e.Transcript == nil {
				return nil
			}
			cp := append([]openai.ChatCompletionMessageParamUnion(nil), e.Transcript...)
			return loop.ToUserVisibleMessages(cp)
		}))
	}
}
