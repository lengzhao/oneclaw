package schedule

import (
	"fmt"
	"strings"

	clawbridge "github.com/lengzhao/clawbridge"
	"github.com/lengzhao/oneclaw/meta"
)

// syntheticScheduleUserPrompt marks the turn as scheduler-fired so the model does not confuse it with live user chat.
func syntheticScheduleUserPrompt(j Job) string {
	body := strings.TrimSpace(j.Prompt)
	id := strings.TrimSpace(j.ID)
	name := strings.TrimSpace(j.Name)
	var b strings.Builder
	b.WriteString("[Scheduler / 定时器任务 — not live user input / 非用户实时输入]\n")
	b.WriteString(fmt.Sprintf("job_id=%s next_run_unix=%d\n", id, j.NextRunUnix))
	if name != "" {
		b.WriteString(fmt.Sprintf("name=%s\n", name))
	}
	b.WriteString("\njob prompt / 任务内容:\n")
	b.WriteString(body)
	return b.String()
}

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
	for k, v := range j.ReplyMeta {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, exists := metaMap[k]; exists {
			continue
		}
		metaMap[k] = v
	}
	sid := strings.TrimSpace(j.SessionSegment)
	return clawbridge.InboundMessage{
		ClientID:  clientID,
		SessionID: sid,
		MessageID: "schedule:" + j.ID,
		Sender:    clawbridge.SenderInfo{Platform: meta.SourceSchedule, DisplayName: "schedule"},
		// Use "direct" like channel drivers (e.g. weixin publishInbound); Reply derives Recipient.Kind from Peer.
		Peer:       clawbridge.Peer{Kind: "direct", ID: sid},
		Content:    syntheticScheduleUserPrompt(j),
		ReceivedAt: j.NextRunUnix,
		Metadata:   metaMap,
	}
}
