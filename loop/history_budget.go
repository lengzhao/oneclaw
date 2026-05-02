package loop

import (
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/budget"
)

// messageTextBudgetBytes is UTF-8 byte length of what the user/assistant/tool “says”: visible text plus
// tool call names and argument JSON on assistant messages (no OpenAI wire JSON wrapper).
func messageTextBudgetBytes(m *schema.Message) int {
	if m == nil {
		return 0
	}
	switch m.Role {
	case schema.User:
		return len(UserMessageText(m)) + userMessageMediaPayloadBytes(m)
	case schema.Tool:
		return len(toolMessageText(m))
	case schema.Assistant:
		n := len(AssistantTextContent(m))
		for _, tc := range m.ToolCalls {
			n += len(tc.Function.Name) + len(tc.Function.Arguments)
		}
		return n
	default:
		return 0
	}
}

func totalMessageTextBytes(msgs []*schema.Message) int {
	n := 0
	for _, m := range msgs {
		n += messageTextBudgetBytes(m)
	}
	return n
}

// TrimMessagesToBudget drops oldest safe prefix until total message text (UTF-8 bytes) fits maxBytes or len<=minKeep.
func TrimMessagesToBudget(msgs []*schema.Message, maxBytes int, minKeep int) []*schema.Message {
	if maxBytes <= 0 || len(msgs) == 0 {
		return msgs
	}
	if minKeep < 0 {
		minKeep = 0
	}
	out := msgs
	for len(out) > minKeep && totalMessageTextBytes(out) > maxBytes {
		next, ok := dropOldestPrefix(out)
		if !ok {
			slog.Warn("loop.budget.cannot_trim_further", "messages", len(out), "text_bytes", totalMessageTextBytes(out), "max", maxBytes)
			break
		}
		out = next
	}
	if len(out) < len(msgs) {
		slog.Info("loop.budget.trim_history", "before", len(msgs), "after", len(out), "text_bytes", totalMessageTextBytes(out), "max", maxBytes)
	}
	return out
}

func dropOldestPrefix(msgs []*schema.Message) ([]*schema.Message, bool) {
	if len(msgs) == 0 {
		return msgs, false
	}
	first := msgs[0]
	if first == nil {
		return msgs[1:], true
	}
	switch first.Role {
	case schema.User:
		return msgs[1:], true
	case schema.Assistant:
		if len(first.ToolCalls) == 0 {
			return msgs[1:], true
		}
		n := len(first.ToolCalls)
		if len(msgs) < 1+n {
			return msgs, false
		}
		for i := 1; i <= n; i++ {
			if msgs[i] == nil || msgs[i].Role != schema.Tool {
				return msgs, false
			}
		}
		return msgs[1+n:], true
	case schema.Tool:
		return msgs[1:], true
	default:
		return msgs, false
	}
}

// ApplyHistoryBudget re-slices *msgs by total UTF-8 text payload bytes (see messageTextBudgetBytes). Optional semantic compact prepends a summary user line.
func ApplyHistoryBudget(g budget.Global, system string, msgs *[]*schema.Message) {
	if msgs == nil || !g.Enabled() {
		return
	}
	limit := g.HistoryByteBudget(len(system))
	full := *msgs
	if semanticCompactEnabled() && len(full) > g.MinTailMessages && limit > 12_000 {
		summaryCap := compactSummaryMaxBytes(limit)
		reserve := summaryCap + 768
		effective := limit - reserve
		if effective > 4096 {
			trimmed := TrimMessagesToBudget(full, effective, g.MinTailMessages)
			if len(trimmed) < len(full) {
				dropped := full[:len(full)-len(trimmed)]
				summary := buildCompactSummary(dropped, summaryCap)
				if strings.TrimSpace(summary) != "" {
					compactMsg := schema.UserMessage(compactEnvelope(summary))
					candidate := append([]*schema.Message{compactMsg}, trimmed...)
					for len(summary) > 64 && totalMessageTextBytes(candidate) > limit {
						summary = utf8TrimToBytes(summary, len(summary)*4/5)
						compactMsg = schema.UserMessage(compactEnvelope(summary))
						candidate = append([]*schema.Message{compactMsg}, trimmed...)
					}
					if totalMessageTextBytes(candidate) <= limit {
						slog.Info("loop.budget.semantic_compact", "dropped_messages", len(dropped), "kept", len(trimmed))
						*msgs = candidate
						return
					}
				}
			}
		}
	}
	*msgs = TrimMessagesToBudget(full, limit, g.MinTailMessages)
}
