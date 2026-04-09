package config

import (
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
		return budget.Global{MaxPromptBytes: 0, MinTailMessages: 4, RecallMaxBytes: 12_000}
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
	if f.Budget.RecallMaxBytes > 0 {
		g.RecallMaxBytes = f.Budget.RecallMaxBytes
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

func clampInt(n, lo, hi, def int) int {
	if n <= 0 {
		return def
	}
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func timeoutFromSec(sec, defSec int) time.Duration {
	if sec <= 0 {
		return time.Duration(defSec) * time.Second
	}
	if sec > 3600 {
		sec = 3600
	}
	return time.Duration(sec) * time.Second
}

// PushRuntime flattens merged YAML into rtopts for packages that cannot import config (e.g. memory).
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
	s.DisableMemoryExtract = featTrue(f.Features.DisableMemoryExtract)
	s.DisableTranscript = featTrue(f.Features.DisableTranscript)
	s.DisableAutoMaintenance = featTrue(f.Features.DisableAutoMaintenance)
	s.DisableScheduledMaintenance = featTrue(f.Features.DisableScheduledMaintenance)
	s.DisableMemoryAudit = featTrue(f.Features.DisableMemoryAudit)

	s.DisableUsageLedger = featTrue(f.Features.DisableUsageLedger)
	s.UsageEstimateCost = featTrue(f.Features.UsageEstimateCost)
	s.DisableBehaviorPolicyWrite = featTrue(f.Features.DisableBehaviorPolicyWrite)
	s.DisableScheduledTasks = featTrue(f.Features.DisableScheduledTasks)
	s.DisableSemanticCompact = featTrue(f.Features.DisableSemanticCompact)
	s.DisableSkills = featTrue(f.Features.DisableSkills)
	s.DisableTasks = featTrue(f.Features.DisableTasks)

	if f.Usage.DefaultInputPerMtok > 0 {
		s.UsageInputPerMtok = f.Usage.DefaultInputPerMtok
	}
	if f.Usage.DefaultOutputPerMtok > 0 {
		s.UsageOutputPerMtok = f.Usage.DefaultOutputPerMtok
	}
	s.ScheduleMinSleep = parseDur(f.Schedule.MinSleep, s.ScheduleMinSleep)
	s.ScheduleIdleSleep = parseDur(f.Schedule.IdleSleep, s.ScheduleIdleSleep)

	s.SidechainMerge = strings.TrimSpace(f.SidechainMerge)
	if f.SemanticCompact.SummaryMaxBytes > 0 {
		s.CompactSummaryMaxBytes = f.SemanticCompact.SummaryMaxBytes
	}
	s.SkillsRecent = strings.TrimSpace(f.Skills.RecentPath)

	m := f.Maintain
	pt := m.PostTurn

	logDays := m.LogDays
	if logDays == 0 && pt.LogDays != 0 {
		logDays = pt.LogDays
	}
	s.MaintenanceLogDays = clampInt(logDays, 1, 14, s.MaintenanceLogDays)

	if m.MinLogBytes > 0 {
		s.MaintenanceMinLogBytes = m.MinLogBytes
	}
	if m.MaxLogReadBytes > 0 {
		s.MaintenanceMaxLogRead = m.MaxLogReadBytes
	}
	if pt.MaxCombinedLogBytes > 0 {
		n := pt.MaxCombinedLogBytes
		if n < 1024 {
			n = 1024
		}
		if n > 256_000 {
			n = 256_000
		}
		s.MaintenanceMaxCombinedLogBytes = n
	}

	if pt.MinLogBytes > 0 {
		s.PostTurnMinLogBytes = pt.MinLogBytes
	}
	if pt.MemoryPreviewBytes > 0 {
		s.PostTurnMemoryPreviewBytes = clampInt(pt.MemoryPreviewBytes, 1024, 24_000, s.PostTurnMemoryPreviewBytes)
	}
	if m.ScheduledTimeoutSeconds > 0 {
		s.ScheduledMaintainTimeout = timeoutFromSec(m.ScheduledTimeoutSeconds, 1800)
	}
	if pt.TimeoutSeconds > 0 {
		s.PostTurnMaintainTimeout = timeoutFromSec(pt.TimeoutSeconds, 60)
	}
	if m.ScheduledMaxSteps > 0 {
		s.ScheduledMaintainMaxSteps = clampInt(m.ScheduledMaxSteps, 2, 64, s.ScheduledMaintainMaxSteps)
	}
	if pt.MaxTopicFiles > 0 {
		s.MaintenanceMaxTopicFiles = clampInt(pt.MaxTopicFiles, 0, 40, s.MaintenanceMaxTopicFiles)
	}
	if pt.TopicExcerptBytes > 0 {
		s.MaintenanceTopicExcerptBytes = clampInt(pt.TopicExcerptBytes, 256, 16_000, s.MaintenanceTopicExcerptBytes)
	}
	s.MaintenanceIncrementalOverlap = parseDur(m.IncrementalOverlap, s.MaintenanceIncrementalOverlap)
	s.MaintenanceIncrementalMaxSpan = parseDur(m.IncrementalMaxSpan, s.MaintenanceIncrementalMaxSpan)

	if pt.UserSnapshotBytes > 0 {
		s.PostTurnUserSnapshotBytes = pt.UserSnapshotBytes
	}
	if pt.AssistantSnapshotBytes > 0 {
		s.PostTurnAssistantSnapshotBytes = pt.AssistantSnapshotBytes
	}
	s.PostTurnLogDays = pt.LogDays
	s.PostTurnMaxCombinedLogBytes = pt.MaxCombinedLogBytes
	s.PostTurnMaxLogBytes = pt.MaxLogBytes
	s.PostTurnMaxTopicFiles = pt.MaxTopicFiles
	s.PostTurnTopicExcerptBytes = pt.TopicExcerptBytes
	s.PostTurnMaxTokens = pt.MaxTokens

	if v := strings.TrimSpace(m.Model); v != "" {
		s.MaintenanceModel = v
	}
	if v := strings.TrimSpace(m.ScheduledModel); v != "" {
		s.MaintenanceScheduledModel = v
	}
	if m.MaxTokens > 0 {
		s.MaintenanceMaxTokens = m.MaxTokens
	}

	rtopts.Set(&s)
}
