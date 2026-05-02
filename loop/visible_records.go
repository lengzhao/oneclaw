package loop

import "github.com/cloudwego/eino/schema"

// ChatTurnRecords converts visible-shaped messages to JSON-friendly {role, content} entries
// (same shape as transcript.json messages).
func ChatTurnRecords(msgs []*schema.Message) []map[string]string {
	out := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		if m == nil {
			continue
		}
		switch m.Role {
		case schema.User:
			t := UserMessageText(m)
			if t == "" {
				continue
			}
			out = append(out, map[string]string{"role": "user", "content": t})
		case schema.Assistant:
			t := AssistantParamText(m)
			if t == "" {
				continue
			}
			out = append(out, map[string]string{"role": "assistant", "content": t})
		default:
			continue
		}
	}
	return out
}

// VisibleTranscriptAppendSince returns role/content records for visible rows that appear after
// the first visibleCountBefore user-visible messages in fullTranscript. Used for notify payloads
// without rewriting the full session history each time.
func VisibleTranscriptAppendSince(fullTranscript []*schema.Message, visibleCountBefore int) []map[string]string {
	vis := ToUserVisibleMessages(fullTranscript)
	if visibleCountBefore < 0 {
		visibleCountBefore = 0
	}
	if visibleCountBefore > len(vis) {
		return nil
	}
	return ChatTurnRecords(vis[visibleCountBefore:])
}
