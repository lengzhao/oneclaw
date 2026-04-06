package routing

import "strings"

// Inbound is wire-neutral metadata for one user turn (see docs/inbound-routing-design.md).
// Text is the user-visible message for this turn (fed to the model by session.Engine.SubmitUser).
// Source is the channel instance id: it must match the key used when registering the Sink for that
// instance (often from config `channels[].id`, e.g. slack1 / slack2).
type Inbound struct {
	Source        string
	Text          string
	Attachments   []Attachment
	Locale        string
	UserID        string
	TenantID      string
	SessionKey    string
	CorrelationID string
	RawRef        any
}

// MergeNonEmptyRouting copies Source, SessionKey, UserID, TenantID, CorrelationID, RawRef from src into dst
// when the corresponding src field is non-empty (after TrimSpace). Text is intentionally not merged.
// Used so loop.RunTurn can refresh routing on the tool context without wiping parent metadata (e.g. subagent turns that only pass Text).
func MergeNonEmptyRouting(dst *Inbound, src Inbound) {
	if dst == nil {
		return
	}
	if s := strings.TrimSpace(src.Source); s != "" {
		dst.Source = s
	}
	if s := strings.TrimSpace(src.SessionKey); s != "" {
		dst.SessionKey = s
	}
	if s := strings.TrimSpace(src.UserID); s != "" {
		dst.UserID = s
	}
	if s := strings.TrimSpace(src.TenantID); s != "" {
		dst.TenantID = s
	}
	if s := strings.TrimSpace(src.CorrelationID); s != "" {
		dst.CorrelationID = s
	}
	if src.RawRef != nil {
		dst.RawRef = src.RawRef
	}
	if s := strings.TrimSpace(src.Locale); s != "" {
		dst.Locale = s
	}
	// Replace attachments each merge: nested RunTurn (e.g. run_agent) passes Text-only Inbound and must
	// not keep the parent user turn's attachments on ToolContext.TurnInbound.
	if len(src.Attachments) > 0 {
		dst.Attachments = append([]Attachment(nil), src.Attachments...)
	} else {
		dst.Attachments = nil
	}
}
