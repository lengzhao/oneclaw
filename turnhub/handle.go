package turnhub

import (
	"strings"

	clawbridge "github.com/lengzhao/clawbridge"

	"github.com/lengzhao/oneclaw/paths"
)

// SessionHandle identifies one mailbox (channel/client × normalized session).
type SessionHandle struct {
	Channel string // typically InboundMessage.ClientID
	Session string // sanitized session segment used under UserDataRoot/sessions/
}

// String returns a stable map key.
func (h SessionHandle) String() string {
	return strings.TrimSpace(h.Channel) + "\x00" + strings.TrimSpace(h.Session)
}

// HandleFromInbound derives a SessionHandle using the same session path rules as CLI --session.
func HandleFromInbound(in *clawbridge.InboundMessage) SessionHandle {
	if in == nil {
		return SessionHandle{}
	}
	return SessionHandle{
		Channel: strings.TrimSpace(in.ClientID),
		Session: paths.SanitizeSessionPathSegment(in.SessionID),
	}
}
