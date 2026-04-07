package loop

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/prompts"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/openai/openai-go"
)

// compactBoundaryKind is the marker string embedded in compact envelopes (persisted transcript).
const compactBoundaryKind = "compact_boundary"

func semanticCompactEnabled() bool {
	return !rtopts.Current().DisableSemanticCompact
}

func compactSummaryMaxBytes(limit int) int {
	if n := rtopts.Current().CompactSummaryMaxBytes; n >= 256 && n <= 64_000 {
		return n
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
	summary = strings.TrimSpace(summary)
	s, err := prompts.Render(prompts.NameCompactEnvelope, struct {
		Kind      string
		Timestamp string
		Summary   string
	}{
		Kind:      compactBoundaryKind,
		Timestamp: ts,
		Summary:   summary,
	})
	if err != nil {
		slog.Warn("loop.prompts.compact_envelope", "err", err)
		return fallbackCompactEnvelope(ts, summary)
	}
	return s
}

func fallbackCompactEnvelope(ts, summary string) string {
	k := compactBoundaryKind
	return "[oneclaw:" + k + " ts=" + ts + "]\n" +
		"Earlier conversation (omitted from context for byte budget). Heuristic recap — verify with tools if needed:\n\n" +
		summary + "\n\n[/oneclaw:" + k + "]\n"
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
