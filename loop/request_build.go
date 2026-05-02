package loop

import "github.com/openai/openai-go"

// BuildRequestMessages returns API messages with an optional leading system message.
// When system is non-empty, it is prepended as [openai.SystemMessage(system)] followed by history.
func BuildRequestMessages(system string, history []openai.ChatCompletionMessageParamUnion) []openai.ChatCompletionMessageParamUnion {
	if system == "" {
		return history
	}
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(history)+1)
	out = append(out, openai.SystemMessage(system))
	out = append(out, history...)
	return out
}
