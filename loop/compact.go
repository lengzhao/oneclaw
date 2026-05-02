package loop

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/prompts"
	"github.com/lengzhao/oneclaw/rtopts"
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

// UserMessageText returns the user-visible UTF-8 text of m, or "" if not a user message.
func UserMessageText(m *schema.Message) string {
	if m == nil || m.Role != schema.User {
		return ""
	}
	if m.Content != "" {
		return m.Content
	}
	var out strings.Builder
	for _, p := range m.UserInputMultiContent {
		if p.Type == schema.ChatMessagePartTypeText && p.Text != "" {
			if out.Len() > 0 {
				out.WriteByte('\n')
			}
			out.WriteString(p.Text)
		}
	}
	for _, p := range m.MultiContent {
		if p.Type == schema.ChatMessagePartTypeText && p.Text != "" {
			if out.Len() > 0 {
				out.WriteByte('\n')
			}
			out.WriteString(p.Text)
		}
	}
	return out.String()
}

// UserMessageHasNonTextMedia reports whether m is a user message carrying image/audio/file parts (not plain string content).
func UserMessageHasNonTextMedia(m *schema.Message) bool {
	if m == nil || m.Role != schema.User {
		return false
	}
	if m.Content != "" {
		return false
	}
	for _, p := range m.UserInputMultiContent {
		switch p.Type {
		case schema.ChatMessagePartTypeText:
			continue
		default:
			return true
		}
	}
	for _, p := range m.MultiContent {
		switch p.Type {
		case schema.ChatMessagePartTypeText:
			continue
		default:
			return true
		}
	}
	return false
}

// userMessageMediaPayloadBytes approximates multimodal payload size for history budgeting (data URLs / base64 audio).
func userMessageMediaPayloadBytes(m *schema.Message) int {
	if m == nil || m.Role != schema.User {
		return 0
	}
	if m.Content != "" {
		return 0
	}
	n := 0
	for _, p := range m.UserInputMultiContent {
		switch p.Type {
		case schema.ChatMessagePartTypeImageURL:
			if p.Image != nil && p.Image.URL != nil {
				n += len(*p.Image.URL)
			}
		case schema.ChatMessagePartTypeAudioURL:
			if p.Audio != nil && p.Audio.Base64Data != nil {
				n += len(*p.Audio.Base64Data)
			}
		case schema.ChatMessagePartTypeFileURL:
			n += 4096
		default:
			n += 256
		}
	}
	for _, p := range m.MultiContent {
		switch p.Type {
		case schema.ChatMessagePartTypeImageURL:
			if p.ImageURL != nil {
				n += len(p.ImageURL.URL)
			}
		case schema.ChatMessagePartTypeAudioURL:
			if p.AudioURL != nil {
				n += len(p.AudioURL.URL) + len(p.AudioURL.URI)
			}
		case schema.ChatMessagePartTypeFileURL:
			n += 4096
		default:
			n += 256
		}
	}
	return n
}

func toolMessageText(m *schema.Message) string {
	if m == nil || m.Role != schema.Tool {
		return ""
	}
	return m.Content
}

// buildCompactSummary turns dropped history into short labeled lines (no extra model call).
func buildCompactSummary(dropped []*schema.Message, maxBytes int) string {
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
		if m == nil {
			continue
		}
		switch m.Role {
		case schema.User:
			t := strings.TrimSpace(UserMessageText(m))
			if t == "" {
				continue
			}
			t = oneLinePreview(t, perLine)
			fmt.Fprintf(&b, "- user: %s\n", t)
		case schema.Assistant:
			t := strings.TrimSpace(AssistantTextContent(m))
			if t == "" && len(m.ToolCalls) > 0 {
				names := make([]string, 0, len(m.ToolCalls))
				for _, tc := range m.ToolCalls {
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
		case schema.Tool:
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
