package loop

import (
	"fmt"
	"strings"

	"github.com/cloudwego/eino/schema"
)

// AssistantTextContent returns assistant-visible text from Content and AssistantGenMultiContent.
func AssistantTextContent(m *schema.Message) string {
	if m == nil || m.Role != schema.Assistant {
		return ""
	}
	if t := strings.TrimSpace(m.Content); t != "" {
		return t
	}
	var b strings.Builder
	for _, part := range m.AssistantGenMultiContent {
		if part.Type == schema.ChatMessagePartTypeText && part.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

// LastAssistantDisplay returns the latest assistant-visible text for CLI/UI and sub-agent results.
// If the last assistant message only requested tools, it summarizes tool names.
func LastAssistantDisplay(msgs []*schema.Message) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m == nil || m.Role != schema.Assistant {
			continue
		}
		if t := strings.TrimSpace(AssistantTextContent(m)); t != "" {
			return t
		}
		if len(m.ToolCalls) > 0 {
			names := make([]string, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				if tc.Function.Name != "" {
					names = append(names, tc.Function.Name)
				}
			}
			if len(names) > 0 {
				return fmt.Sprintf("(已请求工具: %s)", strings.Join(names, ", "))
			}
		}
	}
	return ""
}

// AssistantParamText returns assistant-visible text (content / structured parts), excluding
// tool-call metadata. Empty if m is not an assistant message or has no visible text (e.g. tool-only).
func AssistantParamText(m *schema.Message) string {
	return strings.TrimSpace(AssistantTextContent(m))
}
