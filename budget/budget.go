// Package budget applies a global estimated byte budget to prompts (injections + history).
package budget

import (
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Global configures context sizing for one process. Zero MaxPromptBytes disables enforcement.
type Global struct {
	MaxPromptBytes int
	// MinTailMessages is the minimum messages to keep when trimming history (best-effort).
	MinTailMessages int
}

// FromEnv loads ONCLAW_MAX_PROMPT_BYTES (default 220_000). Set ONCLAW_DISABLE_CONTEXT_BUDGET=1 to disable.
func FromEnv() Global {
	if v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_CONTEXT_BUDGET")); v == "1" || strings.EqualFold(v, "true") {
		return Global{MaxPromptBytes: 0, MinTailMessages: 4}
	}
	maxB := getenvInt("ONCLAW_MAX_PROMPT_BYTES", 220_000)
	tail := getenvInt("ONCLAW_MIN_TRANSCRIPT_MESSAGES", 6)
	if tail < 2 {
		tail = 2
	}
	return Global{MaxPromptBytes: maxB, MinTailMessages: tail}
}

// Enabled is true when history / injection shrinking should run.
func (g Global) Enabled() bool {
	return g.MaxPromptBytes > 0
}

// RecallBytes caps memory recall surfacing (SelectRecall budget).
// Default ceiling matches memory.MaxSurfacedRecallBytes; raise with ONCLAW_RECALL_MAX_BYTES.
func (g Global) RecallBytes() int {
	ceil := getenvInt("ONCLAW_RECALL_MAX_BYTES", 12_000)
	if !g.Enabled() {
		if ceil < 4_000 {
			ceil = 4_000
		}
		return ceil
	}
	n := g.MaxPromptBytes * 18 / 100
	if n < 4_000 {
		n = 4_000
	}
	if n > ceil {
		n = ceil
	}
	return n
}

// InjectCaps returns max bytes for system memory suffix and agentMd block (recall uses RecallBytes separately).
func (g Global) InjectCaps() (systemExtra, agentMd int) {
	if !g.Enabled() {
		return 1 << 30, 1 << 30
	}
	pool := g.MaxPromptBytes * 50 / 100
	if pool < 16_000 {
		pool = 16_000
	}
	// Within injection pool: bias toward MEMORY indices in system suffix.
	systemExtra = pool * 55 / 100
	agentMd = pool * 45 / 100
	return systemExtra, agentMd
}

// HistoryByteBudget is an estimate of how many bytes the transcript (messages only) may use.
func (g Global) HistoryByteBudget(systemLen int) int {
	if !g.Enabled() {
		return 1 << 30
	}
	slack := g.MaxPromptBytes * 8 / 100
	if slack < 8_192 {
		slack = 8_192
	}
	out := g.MaxPromptBytes - systemLen - slack
	if out < 24_000 {
		out = 24_000
	}
	return out
}

// SkillIndexMaxBytes caps the injected "## Skills" listing (names + short descriptions).
// Default ~1% of MaxPromptBytes, clamped; disable when context budget is off (large ceiling).
func (g Global) SkillIndexMaxBytes() int {
	if !g.Enabled() {
		return 1 << 30
	}
	n := g.MaxPromptBytes / 100
	if n < 2048 {
		n = 2048
	}
	if n > 24_000 {
		n = 24_000
	}
	if v := getenvInt("ONCLAW_SKILLS_INDEX_MAX_BYTES", 0); v > 0 {
		return v
	}
	return n
}

// InheritedMessageCap limits fork / run_agent inherit_context tail length.
func (g Global) InheritedMessageCap() int {
	if !g.Enabled() {
		return 48
	}
	// Rough: 22% of prompt / ~700 B per message estimate.
	n := g.MaxPromptBytes * 22 / 100 / 700
	if n < 12 {
		n = 12
	}
	if n > 80 {
		n = 80
	}
	return n
}

// TruncateUTF8 cuts s to at most maxBytes rune-safe and appends a warning line when truncated.
func TruncateUTF8(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return s
	}
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for !utf8.ValidString(s) {
		if len(s) == 0 {
			break
		}
		s = s[:len(s)-1]
	}
	return strings.TrimRight(s, "\n") + "\n\n> WARNING: truncated by global context budget (ONCLAW_MAX_PROMPT_BYTES).\n"
}

func getenvInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}
