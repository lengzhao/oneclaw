package session

import (
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/notify/sinks"
	"github.com/openai/openai-go"
)

// RegisterAuditSinks registers JSONL audit sinks under <InstructionRoot>/audit/<agent_segment>/... .
// Engine.CWD is the per-session workspace (~/.oneclaw/sessions/<id>/); AuditSessionID is left empty so paths do not nest another sessions/<id>.
// Pass true per path to enable that sink; all false is a no-op.
// Agent segment is derived from Engine.RootAgentID (see sinks.SanitizeAgentSegment).
func (e *Engine) RegisterAuditSinks(llm, orchestration, visible bool) {
	if e == nil || (!llm && !orchestration && !visible) {
		return
	}
	auditCWD := e.CWD
	if e.InstructionRoot != "" {
		auditCWD = e.InstructionRoot
	}
	o := sinks.Options{CWD: auditCWD, AgentID: e.EffectiveRootAgentID(), AuditSessionID: "", OmitDotDir: true}
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
