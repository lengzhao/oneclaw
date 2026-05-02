package loop

import (
	"strings"

	"github.com/openai/openai-go"
)

// ToUserVisibleMessages returns a copy of msgs reduced to chat turns a human would see:
// real user lines (attachments kept) and assistant prose. Drops per-turn injections (agentMd,
// recall, routing meta), semantic-compact envelopes, tool rows, and assistants that only issued
// tool calls. Assistant messages that mix visible text with tool_calls are kept as text-only.
// Use after a successful user turn (or when persisting working transcript) to cut token cost; the
// model can re-invoke tools if facts are still needed (cf. file memory / recall).
func ToUserVisibleMessages(msgs []openai.ChatCompletionMessageParamUnion) []openai.ChatCompletionMessageParamUnion {
	if len(msgs) == 0 {
		return msgs
	}
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(msgs))
	for _, m := range msgs {
		switch {
		case m.OfTool != nil:
			continue
		case m.OfUser != nil:
			t := UserMessageText(m)
			hasMedia := UserMessageHasNonTextMedia(m)
			if shouldDropNonVisibleUserText(t) && !hasMedia {
				continue
			}
			if strings.TrimSpace(t) == "" && !hasMedia {
				continue
			}
			out = append(out, m)
		case m.OfAssistant != nil:
			text := AssistantParamText(m)
			nTools := len(m.OfAssistant.ToolCalls)
			if text == "" && nTools > 0 {
				continue
			}
			if text == "" && nTools == 0 {
				continue
			}
			if nTools > 0 {
				out = append(out, openai.AssistantMessage(text))
				continue
			}
			out = append(out, m)
		default:
			continue
		}
	}
	return out
}

func isAgentMdBundleUserText(content string) bool {
	t := strings.TrimSpace(content)
	if t == "" {
		return false
	}
	return strings.Contains(t, "Codebase and user instructions are shown below") &&
		strings.Contains(t, "<system-reminder>") &&
		strings.Contains(t, "# agentMd")
}

func shouldDropNonVisibleUserText(t string) bool {
	if isAgentMdBundleUserText(t) {
		return true
	}
	s := strings.TrimSpace(t)
	if strings.HasPrefix(s, "<inbound-context>") && strings.HasSuffix(s, "</inbound-context>") {
		return true
	}
	if strings.HasPrefix(s, "Attachment: relevant_memories") {
		return true
	}
	if strings.Contains(t, "[oneclaw:compact_boundary") {
		return true
	}
	return false
}
