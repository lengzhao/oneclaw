package schedule

import (
	"strings"

	"github.com/lengzhao/oneclaw/paths"
)

// JobBindingScope identifies which persisted jobs a cron tool list/remove may touch (session + client + catalog agent).
type JobBindingScope struct {
	SessionSegment string
	ClientID       string
	AgentID        string
}

// JobMatchesScope reports whether j belongs to the same isolation scope (normalized comparison).
func JobMatchesScope(j Job, s JobBindingScope) bool {
	j.Normalize()
	sSeg := paths.SanitizeSessionPathSegment(s.SessionSegment)
	jSeg := paths.SanitizeSessionPathSegment(j.SessionSegment)
	if jSeg != sSeg {
		return false
	}
	if strings.TrimSpace(j.ClientID) != strings.TrimSpace(s.ClientID) {
		return false
	}
	if strings.TrimSpace(j.AgentID) != strings.TrimSpace(s.AgentID) {
		return false
	}
	return true
}
