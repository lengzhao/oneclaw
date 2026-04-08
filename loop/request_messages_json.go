package loop

import (
	"encoding/json"

	"github.com/openai/openai-go"
)

// RequestMessagesJSONArray returns a compact JSON array encoding msgs (OpenAI param union per element).
func RequestMessagesJSONArray(msgs []openai.ChatCompletionMessageParamUnion) ([]byte, error) {
	raw := make([]json.RawMessage, len(msgs))
	for i, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		raw[i] = b
	}
	return json.Marshal(raw)
}
