package structuredmem

import (
	"context"
	"strings"

	lzmem "github.com/lengzhao/memory"

	"github.com/lengzhao/oneclaw/paths"
)

// WithIsolationFromTurn attaches lengzhao/memory isolation derived from oneclaw session/catalog ids.
func WithIsolationFromTurn(parent context.Context, sessionSegment, catalogAgentID string) context.Context {
	seg := paths.SanitizeSessionPathSegment(strings.TrimSpace(sessionSegment))
	if seg == "" {
		seg = "default"
	}
	agent := strings.TrimSpace(catalogAgentID)
	if agent == "" {
		agent = "default"
	}
	return lzmem.WithIsolation(parent, "default", seg, seg, agent)
}
