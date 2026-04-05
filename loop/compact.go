package loop

import (
	"fmt"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/message"
	"github.com/openai/openai-go"
)

func semanticCompactEnabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_SEMANTIC_COMPACT"))
	return v != "1" && !strings.EqualFold(v, "true") && !strings.EqualFold(v, "yes")
}

func compactSummaryMaxBytes(limit int) int {
	v := strings.TrimSpace(os.Getenv("ONCLAW_COMPACT_SUMMARY_MAX_BYTES"))
	if v != "" {
		var n int
		_, _ = fmt.Sscanf(v, "%d", &n)
		if n >= 256 && n <= 64_000 {
			return n
		}
	}
	n := limit / 6
	if n < 1024 {
		n = 1024
	}
	if n > 12_000 {
		n = 12_000
	}
	return n
}

func compactEnvelope(summary string) string {
	ts := time.Now().UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("[oneclaw:")
	b.WriteString(string(message.KindCompactBoundary))
	b.WriteString(" ts=")
	b.WriteString(ts)
	b.WriteString("]\n")
	b.WriteString("Earlier conversation (omitted from context for byte budget). Heuristic recap — verify with tools if needed:\n\n")
	b.WriteString(strings.TrimSpace(summary))
	b.WriteString("\n\n[/oneclaw:")
	b.WriteString(string(message.KindCompactBoundary))
	b.WriteString("]\n")
	return b.String()
}

func userMessageText(m openai.ChatCompletionMessageParamUnion) string {
	if m.OfUser == nil {
		return ""
	}
	c := m.OfUser.Content
	if c.OfString.Valid() {
		return c.OfString.Value
	}
	var out strings.Builder
	for _, part := range c.OfArrayOfContentParts {
		if part.OfText != nil && part.OfText.Text != "" {
			if out.Len() > 0 {
				out.WriteByte('\n')
			}
			out.WriteString(part.OfText.Text)
		}
	}
	return out.String()
}

func toolMessageText(m openai.ChatCompletionMessageParamUnion) string {
	if m.OfTool == nil {
		return ""
	}
	c := m.OfTool.Content
	if c.OfString.Valid() {
		return c.OfString.Value
	}
	return ""
}

// buildCompactSummary turns dropped history into short labeled lines (no extra model call).
func buildCompactSummary(dropped []openai.ChatCompletionMessageParamUnion, maxBytes int) string {
	if maxBytes <= 0 || len(dropped) == 0 {
		return ""
	}
	perLine := maxBytes / max(4, len(dropped))
	if perLine < 80 {
		perLine = 80
	}
	if perLine > 600 {
		perLine = 600
	}
	var b strings.Builder
	for _, m := range dropped {
		if b.Len() >= maxBytes {
			break
		}
		switch {
		case m.OfUser != nil:
			t := strings.TrimSpace(userMessageText(m))
			if t == "" {
				continue
			}
			t = oneLinePreview(t, perLine)
			fmt.Fprintf(&b, "- user: %s\n", t)
		case m.OfAssistant != nil:
			t := strings.TrimSpace(assistantContentString(m.OfAssistant))
			if t == "" && m.OfAssistant != nil && len(m.OfAssistant.ToolCalls) > 0 {
				names := make([]string, 0, len(m.OfAssistant.ToolCalls))
				for _, tc := range m.OfAssistant.ToolCalls {
					if tc.Function.Name != "" {
						names = append(names, tc.Function.Name)
					}
				}
				t = fmt.Sprintf("(tool calls: %s)", strings.Join(names, ", "))
			}
			t = oneLinePreview(t, perLine)
			if t != "" {
				fmt.Fprintf(&b, "- assistant: %s\n", t)
			}
		case m.OfTool != nil:
			t := strings.TrimSpace(toolMessageText(m))
			t = oneLinePreview(t, perLine)
			if t != "" {
				fmt.Fprintf(&b, "- tool: %s\n", t)
			}
		}
	}
	s := strings.TrimSpace(b.String())
	if len(s) > maxBytes {
		s = utf8TrimToBytes(s, maxBytes) + "\n…"
	}
	return s
}

func oneLinePreview(s string, maxRunes int) string {
	s = strings.Join(strings.Fields(s), " ")
	if maxRunes <= 0 {
		return ""
	}
	runes := 0
	for i := range s {
		if runes >= maxRunes {
			return s[:i] + "…"
		}
		runes++
	}
	return s
}

func utf8TrimToBytes(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for !utf8.ValidString(s) {
		if len(s) == 0 {
			return ""
		}
		s = s[:len(s)-1]
	}
	return strings.TrimRight(s, "\n")
}
