package schedule

import (
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

func syntheticInboundFromDelivery(clientID string, d TurnDelivery) bus.InboundMessage {
	uid := strings.TrimSpace(d.UserID)
	return bus.InboundMessage{
		Channel:   clientID,
		Content:   d.Text,
		MessageID: d.CorrelationID,
		Peer:      bus.Peer{Kind: "schedule", ID: strings.TrimSpace(d.SessionKey)},
		Sender: bus.SenderInfo{
			PlatformID:  uid,
			CanonicalID: uid,
		},
	}
}
