package sinks

import (
	"strings"
)

const defaultAgentSegment = "_default"
const maxAgentSegmentLen = 64

// SanitizeAgentSegment returns a single path segment safe for .oneclaw/audit/<segment>/...
func SanitizeAgentSegment(agentID string) string {
	s := strings.TrimSpace(agentID)
	if s == "" {
		return defaultAgentSegment
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return defaultAgentSegment
	}
	if len(out) > maxAgentSegmentLen {
		out = out[:maxAgentSegmentLen]
	}
	return out
}

// Options configures audit sinks rooted at CWD.
type Options struct {
	CWD string
	// AgentID derives the audit subdirectory when AgentSegment is empty.
	AgentID string
	// AgentSegment, if non-empty after trim, is sanitized and overrides AgentID for the path.
	AgentSegment string
}

// Segment returns the filesystem directory name under .oneclaw/audit/.
func (o Options) Segment() string {
	if strings.TrimSpace(o.AgentSegment) != "" {
		return SanitizeAgentSegment(o.AgentSegment)
	}
	return SanitizeAgentSegment(o.AgentID)
}
