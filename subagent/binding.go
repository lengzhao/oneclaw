package subagent

import "github.com/lengzhao/oneclaw/paths"

// TurnBinding is an immutable snapshot of channel/session identity for the current turn (tools, observability).
type TurnBinding struct {
	SessionSegment  string
	InboundClientID string
	AgentID         string // resolved catalog agent id for this turn
}

// SanitizedSession returns [paths.SanitizeSessionPathSegment] of SessionSegment.
func (b TurnBinding) SanitizedSession() string {
	return paths.SanitizeSessionPathSegment(b.SessionSegment)
}
