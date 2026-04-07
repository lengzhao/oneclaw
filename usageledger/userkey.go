package usageledger

import (
	"strings"

	"github.com/lengzhao/oneclaw/routing"
)

// UserInteractionKey builds a stable id for per-user rollups (channel user + tenant + source).
func UserInteractionKey(in routing.Inbound) string {
	u := strings.TrimSpace(in.UserID)
	t := strings.TrimSpace(in.TenantID)
	src := strings.TrimSpace(in.Source)
	sk := strings.TrimSpace(in.SessionKey)
	if src == "" {
		src = "unknown"
	}
	if u != "" {
		if t != "" {
			return t + "/" + u + "@" + src
		}
		return u + "@" + src
	}
	if sk != "" {
		return "session:" + sk + "@" + src
	}
	return "anon@" + src
}
