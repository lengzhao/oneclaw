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

// MergeNonEmptyRouting copies Source, SessionKey, UserID, TenantID, CorrelationID, RawRef, Locale from src into dst
// when the corresponding src field is non-empty (after TrimSpace). Text is intentionally not merged.
//
// Attachment rule: if src has no attachments, dst.Attachments is set to nil so nested RunTurn calls (e.g. subagent
// with Text-only Inbound) do not inherit the parent user turn's attachments on ToolContext.TurnInbound.
// Call sites typically use toolctx.Context.ApplyTurnInboundToToolContext, which delegates here.
//
// See docs/inbound-routing-design.md §2.1.
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
	if len(src.Attachments) > 0 {
		dst.Attachments = append([]Attachment(nil), src.Attachments...)
	} else {
		dst.Attachments = nil
	}
}
