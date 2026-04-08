package usageledger

import (
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

// UserInteractionKey builds a stable id for per-user rollups (channel user + tenant + source).
func UserInteractionKey(in bus.InboundMessage) string {
	u := strings.TrimSpace(in.Sender.CanonicalID)
	if u == "" {
		u = strings.TrimSpace(in.Sender.PlatformID)
	}
	t := strings.TrimSpace(in.Sender.Platform)
	src := strings.TrimSpace(in.Channel)
	if src == "" {
		src = "unknown"
	}
	sk := strings.TrimSpace(in.Peer.ID)
	if u != "" {
		if t != "" && t != u {
			return t + "/" + u + "@" + src
		}
		return u + "@" + src
	}
	if sk != "" {
		return "session:" + sk + "@" + src
	}
	if chat := strings.TrimSpace(in.ChatID); chat != "" {
		return "chat:" + chat + "@" + src
	}
	return "anon@" + src
}
