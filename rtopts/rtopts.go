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

	DisableMemory        bool
	DisableAutoMemory    bool
	DisableContextBudget bool

	DisableBehaviorPolicyWrite bool

	DisableScheduledTasks bool
	ScheduleMinSleep      time.Duration
	ScheduleIdleSleep     time.Duration

	SidechainMerge string

	DisableSemanticCompact bool
	CompactSummaryMaxBytes int

	DisableSkills bool
	SkillsRecent  string

	DisableTasks bool

	// ChatCompletionExtraJSON: optional JSON fragment merged into each Chat Completions request before runtime fields (model, messages, tools, …).
	ChatCompletionExtraJSON []byte
}

var cur atomic.Pointer[Snapshot]

// DefaultSnapshot returns the legacy baseline (empty YAML, no env).
func DefaultSnapshot() Snapshot {
	return Snapshot{
		Budget:            budget.DefaultGlobal(),
		ScheduleMinSleep:  time.Second,
		ScheduleIdleSleep: time.Hour,
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
