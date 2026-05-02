package loop

import (
	"strings"

	"github.com/cloudwego/eino/schema"
)

// InboundUserChunk is one user message worth of inbound attachment material after orchestration.
// Either Text (plain user message) or MediaParts (single multimodal user message) is set.
type InboundUserChunk struct {
	Text       string
	MediaParts []schema.MessageInputPart
}

// AppendTurnUserMessages appends memory / inbound-orchestration / user line to the live message list (API history).
// memAgentMd is an optional leading user-shaped block (phase B).
// inboundMeta is optional routing context for the model (correlation_id must stay out — caller responsibility).
// inboundAttachmentChunks are attachment-derived user messages (text hints and/or multimodal parts).
// userLine is the primary user turn text (non-empty after engine validation, except attachment-only placeholder).
func AppendTurnUserMessages(msgs *[]*schema.Message, memAgentMd, inboundMeta string, inboundAttachmentChunks []InboundUserChunk, userLine string) {
	if msgs == nil {
		return
	}
	if s := strings.TrimSpace(memAgentMd); s != "" {
		*msgs = append(*msgs, schema.UserMessage(s))
	}
	if s := strings.TrimSpace(inboundMeta); s != "" {
		*msgs = append(*msgs, schema.UserMessage(s))
	}
	for _, ch := range inboundAttachmentChunks {
		if len(ch.MediaParts) > 0 {
			*msgs = append(*msgs, &schema.Message{
				Role:                  schema.User,
				UserInputMultiContent: ch.MediaParts,
			})
			continue
		}
		if s := strings.TrimSpace(ch.Text); s != "" {
			*msgs = append(*msgs, schema.UserMessage(s))
		}
	}
	*msgs = append(*msgs, schema.UserMessage(userLine))
}
