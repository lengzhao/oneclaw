package schedule

import (
	"strings"

	clawbridge "github.com/lengzhao/clawbridge"
	"github.com/lengzhao/oneclaw/meta"
)

// InboundFromJob builds a synthetic inbound message for the poller (reference-architecture section 2.6).
func InboundFromJob(j Job) clawbridge.InboundMessage {
	j.Normalize()
	clientID := j.ClientID
	if strings.TrimSpace(clientID) == "" {
		clientID = meta.SourceSchedule
	}
	metaMap := map[string]string{
		meta.SourceKey:          meta.SourceSchedule,
		meta.InboundScheduleJob: j.ID,
	}
	if aid := strings.TrimSpace(j.AgentID); aid != "" {
		metaMap[meta.InboundAgent] = aid
	}
	return clawbridge.InboundMessage{
		ClientID:   clientID,
		SessionID:  j.SessionSegment,
		MessageID:  "schedule:" + j.ID,
		Sender:     clawbridge.SenderInfo{Platform: meta.SourceSchedule, DisplayName: "schedule"},
		Peer:       clawbridge.Peer{Kind: "session", ID: j.SessionSegment},
		Content:    j.Prompt,
		ReceivedAt: j.NextRunUnix,
		Metadata:   metaMap,
	}
}
