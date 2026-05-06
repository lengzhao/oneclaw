package wfexec

import (
	"log/slog"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/session"
)

func turnJournalKey(rtx *engine.RuntimeContext) string {
	if rtx == nil {
		return ""
	}
	return session.TurnJournalKey(rtx.CorrelationID, rtx.DelegationDepth, rtx.SessionRoot)
}

func appendRunJournalEntry(rtx *engine.RuntimeContext, phase string, detail map[string]any) {
	if rtx == nil {
		return
	}
	key := turnJournalKey(rtx)
	if key == "" {
		return
	}
	sr := strings.TrimSpace(rtx.EffectiveSessionRoot())
	at := runtimeAgentType(rtx)
	if sr == "" || at == "" {
		return
	}
	if detail == nil {
		detail = map[string]any{}
	}
	if _, ok := detail["correlation_id"]; !ok && strings.TrimSpace(rtx.CorrelationID) != "" {
		detail["correlation_id"] = strings.TrimSpace(rtx.CorrelationID)
	}
	ev := session.RunEvent{
		Ts:        time.Now().UTC(),
		AgentType: at,
		Phase:     phase,
		Detail:    detail,
	}
	ctx := rtx.GoCtx
	if err := session.AppendTurnRunEvent(sr, at, key, ev); err != nil {
		if ctx != nil {
			slog.WarnContext(ctx, "wfexec.run_journal.append_failed", "phase", phase, "err", err)
		} else {
			slog.Warn("wfexec.run_journal.append_failed", "phase", phase, "err", err)
		}
	}
}
