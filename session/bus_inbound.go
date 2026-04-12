package session

import (
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

// InboundSessionKey returns the driver session thread key: bus SessionID when set, else Peer.ID.
func InboundSessionKey(in bus.InboundMessage) string {
	if s := strings.TrimSpace(in.SessionID); s != "" {
		return s
	}
	return strings.TrimSpace(in.Peer.ID)
}

// InboundUserID returns a stable user identifier for policy / usage (CanonicalID, else PlatformID).
func InboundUserID(in bus.InboundMessage) string {
	if s := strings.TrimSpace(in.Sender.CanonicalID); s != "" {
		return s
	}
	return strings.TrimSpace(in.Sender.PlatformID)
}

// InboundTenantHint returns a coarse tenant/workspace hint (Sender.Platform), may be empty.
func InboundTenantHint(in bus.InboundMessage) string {
	return strings.TrimSpace(in.Sender.Platform)
}

// assistantTextOutbound builds a single assistant text message back to the same chat/thread as the turn.
func assistantTextOutbound(turn *bus.InboundMessage, text string) *bus.OutboundMessage {
	if turn == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	if strings.TrimSpace(turn.ClientID) == "" || strings.TrimSpace(turn.SessionID) == "" {
		return nil
	}
	return &bus.OutboundMessage{
		ClientID:  turn.ClientID,
		To:        bus.Recipient{SessionID: turn.SessionID, Kind: turn.Peer.Kind},
		Text:      text,
		ReplyToID: turn.MessageID,
	}
}

// assistantOutboundWithMedia appends MediaPart entries (paths from session attachments after persistence).
func assistantOutboundWithMedia(turn *bus.InboundMessage, text string, parts []bus.MediaPart) *bus.OutboundMessage {
	if turn == nil {
		return nil
	}
	msg := assistantTextOutbound(turn, text)
	if msg == nil && len(parts) == 0 {
		return nil
	}
	if msg == nil {
		if strings.TrimSpace(turn.ClientID) == "" || strings.TrimSpace(turn.SessionID) == "" {
			return nil
		}
		msg = &bus.OutboundMessage{
			ClientID:  turn.ClientID,
			To:        bus.Recipient{SessionID: turn.SessionID, Kind: turn.Peer.Kind},
			ReplyToID: turn.MessageID,
		}
	}
	msg.Parts = append([]bus.MediaPart(nil), parts...)
	return msg
}

// InboundUpdateStatusRequest builds a clawbridge per-message status update for the
// triggering inbound, or nil when MessageID / addressing is missing (driver cannot target the row).
func InboundUpdateStatusRequest(in *bus.InboundMessage, state string) *bus.UpdateStatusRequest {
	if in == nil || strings.TrimSpace(state) == "" {
		return nil
	}
	if strings.TrimSpace(in.MessageID) == "" {
		return nil
	}
	if strings.TrimSpace(in.ClientID) == "" || strings.TrimSpace(in.SessionID) == "" {
		return nil
	}
	return &bus.UpdateStatusRequest{
		ClientID:  strings.TrimSpace(in.ClientID),
		To:        bus.Recipient{SessionID: strings.TrimSpace(in.SessionID), Kind: in.Peer.Kind},
		MessageID: strings.TrimSpace(in.MessageID),
		State:     state,
	}
}
