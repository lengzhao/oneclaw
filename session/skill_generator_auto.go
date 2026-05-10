package session

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
)

const skillGeneratorAutoTimeout = 45 * time.Minute

// maybeAutoSkillGeneratorAfterTurn runs skill-generator asynchronously when the turn matches
// [memory.PostTurnSkillMaintainTrigger] (heavy tool use or invoke_skill), the catalog defines
// skill-generator, and skills / auto hook are not disabled.
func (e *Engine) maybeAutoSkillGeneratorAfterTurn(prep sharedTurnPrep, traceSink *loop.ToolTraceSink, userPreview, correlationID string) {
	if rtopts.Current().DisableAutoSkillGenerator || rtopts.Current().DisableSkills {
		return
	}
	if prep.catalog == nil {
		return
	}
	if _, ok := prep.catalog.Get("skill-generator"); !ok {
		return
	}
	var tools []loop.ToolTraceEntry
	if traceSink != nil {
		tools = traceSink.Snapshot()
	}
	if !memory.PostTurnSkillMaintainTrigger(&memory.PostTurnInput{Tools: tools}) {
		return
	}
	task := memory.SkillGeneratorAutoTask(userPreview, loop.LastAssistantDisplay(e.Messages),
		strings.TrimSpace(e.SessionID), strings.TrimSpace(correlationID), tools)
	slog.Info("session.skill_generator.auto_scheduled", "session_id", e.SessionID, "tool_calls", len(tools))
	go e.runAutoSkillGenerator(context.Background(), prep, task)
}

func (e *Engine) runAutoSkillGenerator(bg context.Context, prep sharedTurnPrep, task string) {
	ctx, cancel := context.WithTimeout(bg, skillGeneratorAutoTimeout)
	defer cancel()
	if prep.tctx == nil || prep.tctx.Subagent == nil {
		return
	}
	reply, err := prep.tctx.Subagent.RunAgent(ctx, prep.tctx, "skill-generator", task, false)
	if err != nil {
		slog.Warn("session.skill_generator.auto_failed", "session_id", e.SessionID, "err", err)
		return
	}
	runes := []rune(strings.TrimSpace(reply))
	prev := string(runes)
	if len(runes) > 500 {
		prev = string(runes[:500]) + "…"
	}
	slog.Info("session.skill_generator.auto_done", "session_id", e.SessionID, "reply_preview", prev)
}
