package session

import (
	"encoding/json"
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

// SendMessageRoutingArgs captures optional routing overrides from the send_message tool (clawbridge-aligned).
type SendMessageRoutingArgs struct {
	ClientID  string
	SessionID string
	PeerKind  string
	PeerID    string
	ToUserID  string
	TenantID  string
}

// SendMessageToolRoutingFromJSON parses send_message tool JSON for routing gates (partial OK).
// Accepts legacy keys: source → client_id, session_key → session_id, user_id → to_user_id.
func SendMessageToolRoutingFromJSON(raw json.RawMessage) SendMessageRoutingArgs {
	var s struct {
		ClientID   string `json:"client_id"`
		SessionID  string `json:"session_id"`
		PeerKind   string `json:"peer_kind"`
		PeerID     string `json:"peer_id"`
		ToUserID   string `json:"to_user_id"`
		TenantID   string `json:"tenant_id"`
		Source     string `json:"source"`
		SessionKey string `json:"session_key"`
		UserID     string `json:"user_id"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &s)
	}
	a := SendMessageRoutingArgs{
		ClientID:  strings.TrimSpace(s.ClientID),
		SessionID: strings.TrimSpace(s.SessionID),
		PeerKind:  strings.TrimSpace(s.PeerKind),
		PeerID:    strings.TrimSpace(s.PeerID),
		ToUserID:  strings.TrimSpace(s.ToUserID),
		TenantID:  strings.TrimSpace(s.TenantID),
	}
	if a.ClientID == "" {
		a.ClientID = strings.TrimSpace(s.Source)
	}
	if a.SessionID == "" {
		a.SessionID = strings.TrimSpace(s.SessionKey)
	}
	if a.ToUserID == "" {
		a.ToUserID = strings.TrimSpace(s.UserID)
	}
	return a
}

// SendMessageTargetOverridesTurn is true when args explicitly target a different route than tin
// (another clawbridge client, session, peer endpoint, recipient user, or tenant hint).
func SendMessageTargetOverridesTurn(tin bus.InboundMessage, a SendMessageRoutingArgs) bool {
	if a.ClientID != "" && a.ClientID != strings.TrimSpace(tin.ClientID) {
		return true
	}
	if a.SessionID != "" && a.SessionID != strings.TrimSpace(tin.SessionID) {
		return true
	}
	if a.PeerID != "" && a.PeerID != strings.TrimSpace(tin.Peer.ID) {
		return true
	}
	if a.PeerKind != "" && a.PeerKind != strings.TrimSpace(tin.Peer.Kind) {
		return true
	}
	uid := InboundUserID(tin)
	if a.ToUserID != "" && a.ToUserID != uid {
		return true
	}
	th := InboundTenantHint(tin)
	if a.TenantID != "" && a.TenantID != th {
		return true
	}
	return false
}
