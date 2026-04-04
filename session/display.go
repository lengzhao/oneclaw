package session

import (
	"fmt"
	"strings"

	"github.com/openai/openai-go"
)

// LastAssistantDisplay returns the latest assistant-visible text for CLI/UI.
// If the last assistant message only requested tools, it summarizes tool names.
func LastAssistantDisplay(msgs []openai.ChatCompletionMessageParamUnion) string {
	for i := len(msgs) - 1; i >= 0; i-- {
		u := msgs[i]
		if u.OfAssistant == nil {
			continue
		}
		a := u.OfAssistant
		if a.Refusal.Valid() && a.Refusal.Value != "" {
			return "[refusal] " + a.Refusal.Value
		}
		if t := assistantContentString(a); t != "" {
			return t
		}
		if len(a.ToolCalls) > 0 {
			names := make([]string, 0, len(a.ToolCalls))
			for _, tc := range a.ToolCalls {
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

func assistantContentString(a *openai.ChatCompletionAssistantMessageParam) string {
	if a == nil {
		return ""
	}
	c := a.Content
	if c.OfString.Valid() && c.OfString.Value != "" {
		return c.OfString.Value
	}
	var b strings.Builder
	for _, part := range c.OfArrayOfContentParts {
		if part.OfText != nil && part.OfText.Text != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(part.OfText.Text)
		}
		if part.OfRefusal != nil && part.OfRefusal.Refusal != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(part.OfRefusal.Refusal)
		}
	}
	return b.String()
}
