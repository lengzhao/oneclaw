// Package message defines transcript-level markers beyond raw API messages.
// Phase A: compact / attachment roles are reserved; wire messages use openai.ChatCompletionMessageParamUnion (see loop/session).
package message

// Kind marks optional envelope fields when extending persisted transcripts.
type Kind string

const (
	KindUser             Kind = "user"
	KindAssistant        Kind = "assistant"
	KindToolResult       Kind = "tool_result"
	KindAttachment       Kind = "attachment"
	KindCompactBoundary  Kind = "compact_boundary" // placeholder for future compaction
)
