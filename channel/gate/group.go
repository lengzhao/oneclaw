package gate

import "strings"

// GroupTrigger mirrors picoclaw group_trigger (prefixes + mention-only).
type GroupTrigger struct {
	MentionOnly bool
	Prefixes    []string
}

// ShouldRespondInGroup decides whether to handle a group message after platform-specific mention stripping.
func ShouldRespondInGroup(gt GroupTrigger, isMentioned bool, content string) (respond bool, cleaned string) {
	content = strings.TrimSpace(content)
	if isMentioned {
		return true, content
	}
	if gt.MentionOnly {
		return false, content
	}
	if len(gt.Prefixes) > 0 {
		for _, prefix := range gt.Prefixes {
			if prefix != "" && strings.HasPrefix(content, prefix) {
				return true, strings.TrimSpace(strings.TrimPrefix(content, prefix))
			}
		}
		return false, content
	}
	return true, content
}

// IsAllowed checks allow_from rules (empty list = allow all; "*" = explicit open).
func IsAllowed(allowFrom []string, senderID string) bool {
	if len(allowFrom) == 0 {
		return true
	}
	idPart := senderID
	userPart := ""
	if idx := strings.Index(senderID, "|"); idx > 0 {
		idPart = senderID[:idx]
		userPart = senderID[idx+1:]
	}
	for _, allowed := range allowFrom {
		if allowed == "*" {
			return true
		}
		trimmed := strings.TrimPrefix(allowed, "@")
		allowedID := trimmed
		allowedUser := ""
		if idx := strings.Index(trimmed, "|"); idx > 0 {
			allowedID = trimmed[:idx]
			allowedUser = trimmed[idx+1:]
		}
		if senderID == allowed ||
			idPart == allowed ||
			senderID == trimmed ||
			idPart == trimmed ||
			idPart == allowedID ||
			(allowedUser != "" && senderID == allowedUser) ||
			(userPart != "" && (userPart == allowed || userPart == trimmed || userPart == allowedUser)) {
			return true
		}
	}
	return false
}
