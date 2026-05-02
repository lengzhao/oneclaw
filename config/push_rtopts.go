package config

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/rtopts"
)

func featTrue(b *bool) bool {
	return b != nil && *b
}

func buildBudgetGlobal(f File) budget.Global {
	if featTrue(f.Features.DisableContextBudget) {
		return budget.Global{MaxPromptBytes: 0, MinTailMessages: 4}
	}
	g := budget.DefaultGlobal()
	if f.Budget.MaxPromptBytes > 0 {
		g.MaxPromptBytes = f.Budget.MaxPromptBytes
	} else if f.Budget.MaxContextTokens > 0 {
		tok := f.Budget.MaxContextTokens
		if tok < 1 {
			tok = 110_000
		}
		g.MaxPromptBytes = tok * 2
	}
	if f.Budget.MinTranscriptMessages > 0 {
		g.MinTailMessages = f.Budget.MinTranscriptMessages
		if g.MinTailMessages < 2 {
			g.MinTailMessages = 2
		}
	}
	if f.Budget.HistoryMaxBytes != 0 {
		g.HistoryMaxBytes = f.Budget.HistoryMaxBytes
	}
	if f.Budget.SystemExtraMaxBytes != 0 {
		g.SystemExtraMaxBytes = f.Budget.SystemExtraMaxBytes
	}
	if f.Budget.AgentMdMaxBytes != 0 {
		g.AgentMdMaxBytes = f.Budget.AgentMdMaxBytes
	}
	if f.Budget.SkillIndexMaxBytes != 0 {
		g.SkillIndexBytes = f.Budget.SkillIndexMaxBytes
	}
	if f.Budget.InheritedMessages != 0 {
		g.InheritedMessages = f.Budget.InheritedMessages
	}
	return g
}

func parseDur(s string, def time.Duration) time.Duration {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	d, err := time.ParseDuration(s)
	if err != nil || d < 0 {
		return def
	}
	return d
}

// PushRuntime flattens merged YAML into rtopts for packages that cannot import config.
func (r *Resolved) PushRuntime() {
	if r == nil {
		rtopts.Set(nil)
		return
	}
	s := rtopts.DefaultSnapshot()
	f := r.merged

	s.Budget = buildBudgetGlobal(f)
	s.MemoryBase = strings.TrimSpace(f.Paths.MemoryBase)
	s.ChatTransport = strings.TrimSpace(f.Chat.Transport)

	s.DisableMemory = featTrue(f.Features.DisableMemory)
	s.DisableAutoMemory = featTrue(f.Features.DisableAutoMemory)

	s.DisableBehaviorPolicyWrite = featTrue(f.Features.DisableBehaviorPolicyWrite)
	s.DisableScheduledTasks = featTrue(f.Features.DisableScheduledTasks)
	s.DisableSemanticCompact = featTrue(f.Features.DisableSemanticCompact)
	s.DisableSkills = featTrue(f.Features.DisableSkills)
	s.DisableTasks = featTrue(f.Features.DisableTasks)

	s.ScheduleMinSleep = parseDur(f.Schedule.MinSleep, s.ScheduleMinSleep)
	s.ScheduleIdleSleep = parseDur(f.Schedule.IdleSleep, s.ScheduleIdleSleep)

	s.SidechainMerge = strings.TrimSpace(f.SidechainMerge)
	if f.SemanticCompact.SummaryMaxBytes > 0 {
		s.CompactSummaryMaxBytes = f.SemanticCompact.SummaryMaxBytes
	}
	s.SkillsRecent = strings.TrimSpace(f.Skills.RecentPath)
	s.ChatCompletionExtraJSON = nil
	if len(f.Agent.CompletionExtra) > 0 {
		b, err := json.Marshal(f.Agent.CompletionExtra)
		if err != nil {
			slog.Warn("config.completion_extra.marshal_failed", "err", err)
		} else {
			s.ChatCompletionExtraJSON = b
		}
	}

	rtopts.Set(&s)
}
