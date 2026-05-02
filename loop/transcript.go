package loop

import (
	"encoding/json"

	"github.com/openai/openai-go"
)

// TranscriptJSON is a serializable view of the conversation (API-shaped messages).
type TranscriptJSON struct {
	Messages []json.RawMessage `json:"messages"`
}

// MarshalMessages encodes message params to JSON for persistence.
func MarshalMessages(msgs []openai.ChatCompletionMessageParamUnion) ([]byte, error) {
	raw := make([]json.RawMessage, len(msgs))
	for i, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		raw[i] = b
	}
	return json.MarshalIndent(TranscriptJSON{Messages: raw}, "", "  ")
}

// UnmarshalMessages decodes transcript bytes into message slice.
func UnmarshalMessages(data []byte) ([]openai.ChatCompletionMessageParamUnion, error) {
	var wrap TranscriptJSON
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, err
	}
	out := make([]openai.ChatCompletionMessageParamUnion, len(wrap.Messages))
	for i, r := range wrap.Messages {
		if err := json.Unmarshal(r, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
