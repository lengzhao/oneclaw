package loop

import (
	"strings"

	"github.com/openai/openai-go"
)

// InboundUserChunk is one user message worth of inbound attachment material after orchestration.
// Either Text (plain user message) or MediaParts (single multimodal user message) is set.
type InboundUserChunk struct {
	Text       string
	MediaParts []openai.ChatCompletionContentPartUnionParam
}

// AppendTurnUserMessages appends memory / inbound-orchestration / user line to the live message list (API history).
// memAgentMd and memRecall are optional leading user-shaped blocks (phase B).
// inboundMeta is optional routing context for the model (correlation_id must stay out — caller responsibility).
// inboundAttachmentChunks are attachment-derived user messages (text hints and/or multimodal parts).
// userLine is the primary user turn text (non-empty after engine validation, except attachment-only placeholder).
func AppendTurnUserMessages(msgs *[]openai.ChatCompletionMessageParamUnion, memAgentMd, memRecall, inboundMeta string, inboundAttachmentChunks []InboundUserChunk, userLine string) {
	if msgs == nil {
		return
	}
	if s := strings.TrimSpace(memAgentMd); s != "" {
		*msgs = append(*msgs, openai.UserMessage(s))
	}
	if s := strings.TrimSpace(memRecall); s != "" {
		*msgs = append(*msgs, openai.UserMessage(s))
	}
	if s := strings.TrimSpace(inboundMeta); s != "" {
		*msgs = append(*msgs, openai.UserMessage(s))
	}
	for _, ch := range inboundAttachmentChunks {
		if len(ch.MediaParts) > 0 {
			*msgs = append(*msgs, openai.UserMessage(ch.MediaParts))
			continue
		}
		if s := strings.TrimSpace(ch.Text); s != "" {
			*msgs = append(*msgs, openai.UserMessage(s))
		}
	}
	*msgs = append(*msgs, openai.UserMessage(userLine))
}
