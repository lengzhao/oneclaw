package schedule

import (
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

func syntheticInboundFromDelivery(clientID string, d TurnDelivery) bus.InboundMessage {
	uid := strings.TrimSpace(d.UserID)
	pk := strings.TrimSpace(d.PeerKind)
	if pk == "" {
		pk = "schedule"
	}
	return bus.InboundMessage{
		Channel:   clientID,
		ChatID:    strings.TrimSpace(d.TargetChatID),
		Content:   d.Text,
		MessageID: d.CorrelationID,
		Peer:      bus.Peer{Kind: pk, ID: strings.TrimSpace(d.SessionKey)},
		Sender: bus.SenderInfo{
			PlatformID:  uid,
			CanonicalID: uid,
		},
	}
}
