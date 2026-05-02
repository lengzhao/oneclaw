package loop

import (
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/schema"
)

// TranscriptJSON is a serializable view of the conversation (schema.Message per element).
type TranscriptJSON struct {
	Messages []json.RawMessage `json:"messages"`
}

// MarshalMessages encodes messages to JSON for persistence.
func MarshalMessages(msgs []*schema.Message) ([]byte, error) {
	raw := make([]json.RawMessage, 0, len(msgs))
	for _, m := range msgs {
		if m == nil {
			continue
		}
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		raw = append(raw, b)
	}
	return json.MarshalIndent(TranscriptJSON{Messages: raw}, "", "  ")
}

// UnmarshalMessages decodes transcript bytes into schema messages (same format as MarshalMessages).
func UnmarshalMessages(data []byte) ([]*schema.Message, error) {
	var wrap TranscriptJSON
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, err
	}
	out := make([]*schema.Message, 0, len(wrap.Messages))
	for i, r := range wrap.Messages {
		var sm schema.Message
		if err := json.Unmarshal(r, &sm); err != nil {
			return nil, fmt.Errorf("message %d: %w", i, err)
		}
		if sm.Role == "" {
			return nil, fmt.Errorf("message %d: missing role", i)
		}
		cp := sm
		out = append(out, &cp)
	}
	return out, nil
}
