// Package budget applies UTF-8 byte limits to prompts (injections + history).
package budget

import (
	"strings"
	"unicode/utf8"
)

const defaultContextTokens = 110_000

// Global holds byte caps. MaxPromptBytes is the overall context text budget (explicit bytes or token×2 from config).
// Segment fields > 0 override defaults; 0 means use a simple fraction of MaxPromptBytes.
type Global struct {
	MaxPromptBytes    int
	MinTailMessages   int
	RecallMaxBytes    int // ceiling for RecallBytes(); 0 means 12_000
	HistoryMaxBytes   int
	SystemExtraMaxBytes int
	AgentMdMaxBytes   int
	SkillIndexBytes   int
	InheritedMessages int
}

// DefaultGlobal matches the historical default when YAML omits budget (110000 tokens ×2, etc.).
func DefaultGlobal() Global {
	maxB := defaultContextTokens * 2
	return Global{
		MaxPromptBytes:    maxB,
		MinTailMessages:   6,
		RecallMaxBytes:    12_000,
		HistoryMaxBytes:   0,
		SystemExtraMaxBytes: 0,
		AgentMdMaxBytes:   0,
		SkillIndexBytes:   0,
		InheritedMessages: 0,
	}
}

func (g Global) Enabled() bool { return g.MaxPromptBytes > 0 }

// RecallBytes caps memory recall (SelectRecall). Uses RecallMaxBytes as ceiling (default 12_000); default share of MaxPromptBytes when enabled.
func (g Global) RecallBytes() int {
	ceil := g.RecallMaxBytes
	if ceil <= 0 {
		ceil = 12_000
	}
	if !g.Enabled() {
		if ceil < 4_000 {
			ceil = 4_000
		}
		return ceil
	}
	n := g.MaxPromptBytes * 10 / 100
	if n < 4_000 {
		n = 4_000
	}
	if n > ceil {
		n = ceil
	}
	return n
}

// InjectCaps returns max bytes for system memory suffix and agentMd block.
func (g Global) InjectCaps() (systemExtra, agentMd int) {
	if !g.Enabled() {
		return 1 << 30, 1 << 30
	}
	sys := g.SystemExtraMaxBytes
	if sys <= 0 {
		sys = g.MaxPromptBytes * 20 / 100
		if sys < 8_000 {
			sys = 8_000
		}
	}
	ag := g.AgentMdMaxBytes
	if ag <= 0 {
		ag = g.MaxPromptBytes * 22 / 100
		if ag < 8_000 {
			ag = 8_000
		}
	}
	return sys, ag
}

// HistoryByteBudget is max UTF-8 bytes for transcript message text payloads (user/assistant/tool content; assistant tool names+args), aligned with len(system).
func (g Global) HistoryByteBudget(systemLen int) int {
	if !g.Enabled() {
		return 1 << 30
	}
	slack := g.MaxPromptBytes * 5 / 100
	if slack < 4_096 {
		slack = 4_096
	}
	room := g.MaxPromptBytes - systemLen - slack
	if room < 8_000 {
		room = 8_000
	}
	h := g.HistoryMaxBytes
	if h <= 0 {
		h = g.MaxPromptBytes * 42 / 100
		if h < 24_000 {
			h = 24_000
		}
	}
	if h > room {
		return room
	}
	return h
}

// SkillIndexMaxBytes caps the "## Skills" / catalog listing.
func (g Global) SkillIndexMaxBytes() int {
	if g.SkillIndexBytes > 0 {
		return g.SkillIndexBytes
	}
	if !g.Enabled() {
		return 1 << 30
	}
	n := g.MaxPromptBytes / 100
	if n < 2_048 {
		n = 2_048
	}
	if n > 24_000 {
		n = 24_000
	}
	return n
}

// InheritedMessageCap limits fork / run_agent inherit_context tail (message count).
func (g Global) InheritedMessageCap() int {
	if g.InheritedMessages > 0 {
		return g.InheritedMessages
	}
	if !g.Enabled() {
		return 48
	}
	n := g.MaxPromptBytes / 800
	if n < 12 {
		n = 12
	}
	if n > 80 {
		n = 80
	}
	return n
}

// TruncateUTF8 cuts s to at most maxBytes rune-safe and appends a warning when truncated.
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
	return strings.TrimRight(s, "\n") + "\n\n> WARNING: truncated by context byte limit (budget).\n"
}
