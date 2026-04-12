package schedule

import (
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

// MetadataKeySyntheticScheduleFire marks bus.InboundMessage produced by the host schedule poller.
// Host/tool layers use it to avoid duplicate outbound (e.g. model calling send_message for text
// the user already received as the injected user turn).
const MetadataKeySyntheticScheduleFire = "oneclaw_synthetic_schedule"

// IsSyntheticScheduleInbound reports whether in was injected by CollectDue / host poller.
func IsSyntheticScheduleInbound(in *bus.InboundMessage) bool {
	if in == nil || in.Metadata == nil {
		return false
	}
	_, ok := in.Metadata[MetadataKeySyntheticScheduleFire]
	return ok
}

func syntheticInboundFromDelivery(clientID string, d TurnDelivery) bus.InboundMessage {
	uid := strings.TrimSpace(d.UserID)
	pk := strings.TrimSpace(d.PeerKind)
	if pk == "" {
		pk = "schedule"
	}
	return bus.InboundMessage{
		ClientID:  clientID,
		SessionID: strings.TrimSpace(d.TargetSessionID),
		Content:   d.Text,
		MessageID: d.CorrelationID,
		Peer:      bus.Peer{Kind: pk, ID: strings.TrimSpace(d.SessionKey)},
		Sender: bus.SenderInfo{
			PlatformID:  uid,
			CanonicalID: uid,
		},
		Metadata: map[string]string{
			MetadataKeySyntheticScheduleFire: "1",
		},
	}
}
