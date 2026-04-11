package loop

import "github.com/openai/openai-go"

// ChatTurnRecords converts visible-shaped messages to JSON-friendly {role, content} entries
// (same shape as transcript.json messages).
func ChatTurnRecords(msgs []openai.ChatCompletionMessageParamUnion) []map[string]string {
	out := make([]map[string]string, 0, len(msgs))
	for _, m := range msgs {
		switch {
		case m.OfUser != nil:
			t := UserMessageText(m)
			if t == "" {
				continue
			}
			out = append(out, map[string]string{"role": "user", "content": t})
		case m.OfAssistant != nil:
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
// the first visibleCountBefore user-visible messages in fullTranscript. Used to audit one turn
// without rewriting the full session history each time.
func VisibleTranscriptAppendSince(fullTranscript []openai.ChatCompletionMessageParamUnion, visibleCountBefore int) []map[string]string {
	vis := ToUserVisibleMessages(fullTranscript)
	if visibleCountBefore < 0 {
		visibleCountBefore = 0
	}
	if visibleCountBefore > len(vis) {
		return nil
	}
	return ChatTurnRecords(vis[visibleCountBefore:])
}
