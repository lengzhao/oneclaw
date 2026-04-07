// Package rtopts holds process-wide runtime options merged from YAML (see config.Load + PushRuntime).
// Callers read Current(); tests may Set(nil) to reset or pass a custom Snapshot.
package rtopts

import (
	"sync/atomic"
	"time"

	"github.com/lengzhao/oneclaw/budget"
)

// Snapshot is a flattened view of oneclaw YAML + defaults. Zero value matches legacy “no env overrides”.
type Snapshot struct {
	Budget budget.Global

	MemoryBase string

	ChatTransport string

	DisableMemory              bool
	DisableAutoMemory          bool
	DisableMemoryExtract       bool
	DisableTurnLog             bool
	DisableTranscript          bool
	DisableAutoMaintenance     bool
	DisableScheduledMaintenance bool
	DisableMemoryAudit         bool
	DisableContextBudget       bool

	DisableUsageLedger    bool
	UsageEstimateCost     bool
	UsageInputPerMtok     float64
	UsageOutputPerMtok    float64

	DisableBehaviorPolicyWrite bool

	DisableScheduledTasks bool
	ScheduleMinSleep      time.Duration
	ScheduleIdleSleep     time.Duration

	SidechainMerge string

	DisableSemanticCompact  bool
	CompactSummaryMaxBytes  int

	DisableSkills bool
	SkillsRecent  string

	DisableTasks bool

	TurnLogPath string

	// Scheduled / post-turn maintenance tuning (defaults in DefaultSnapshot; YAML via PushRuntime).
	MaintenanceLogDays              int
	MaintenanceMinLogBytes          int
	MaintenanceMaxLogRead           int
	MaintenanceMaxCombinedLogBytes  int
	PostTurnMinLogBytes             int
	PostTurnMemoryPreviewBytes      int
	ScheduledMaintainTimeout        time.Duration
	PostTurnMaintainTimeout         time.Duration
	ScheduledMaintainMaxSteps       int
	MaintenanceMaxTopicFiles        int
	MaintenanceTopicExcerptBytes    int
	MaintenanceIncrementalOverlap   time.Duration
	MaintenanceIncrementalMaxSpan   time.Duration
	PostTurnUserSnapshotBytes       int
	PostTurnAssistantSnapshotBytes  int
	PostTurnLogDays                 int
	PostTurnMaxCombinedLogBytes     int
	PostTurnMaxLogBytes             int
	PostTurnMaxTopicFiles           int
	PostTurnTopicExcerptBytes       int
	PostTurnMaxTokens               int64

	MaintenanceModel          string
	MaintenanceScheduledModel string
	MaintenanceMaxTokens      int64
}

var cur atomic.Pointer[Snapshot]

// DefaultSnapshot returns the legacy baseline (empty YAML, no env).
func DefaultSnapshot() Snapshot {
	return Snapshot{
		Budget:                         budget.DefaultGlobal(),
		UsageInputPerMtok:              5,
		UsageOutputPerMtok:             15,
		ScheduleMinSleep:               time.Second,
		ScheduleIdleSleep:              time.Hour,
		MaintenanceLogDays:             3,
		MaintenanceMinLogBytes:         200,
		MaintenanceMaxLogRead:          24_000,
		MaintenanceMaxCombinedLogBytes: 48_000,
		PostTurnMinLogBytes:            200,
		PostTurnMemoryPreviewBytes:     4000,
		ScheduledMaintainTimeout:       1800 * time.Second,
		PostTurnMaintainTimeout:        60 * time.Second,
		ScheduledMaintainMaxSteps:      24,
		MaintenanceMaxTopicFiles:       12,
		MaintenanceTopicExcerptBytes:   2048,
		MaintenanceIncrementalOverlap:  2 * time.Minute,
		MaintenanceIncrementalMaxSpan:  168 * time.Hour,
		PostTurnUserSnapshotBytes:      4000,
		PostTurnAssistantSnapshotBytes: 8000,
		PostTurnLogDays:                0,
	}
}

// Current returns the active snapshot, or DefaultSnapshot if Set has not been called.
func Current() Snapshot {
	p := cur.Load()
	if p == nil {
		return DefaultSnapshot()
	}
	return *p
}

// Set replaces the process-wide snapshot. Pass nil to clear (Current() falls back to DefaultSnapshot).
func Set(s *Snapshot) {
	if s == nil {
		cur.Store(nil)
		return
	}
	cp := *s
	cur.Store(&cp)
}
