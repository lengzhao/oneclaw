package loop

import (
	"encoding/json"

	"github.com/cloudwego/eino/schema"
)

// IsUserMessage reports whether m is a non-nil user-role message.
func IsUserMessage(m *schema.Message) bool {
	return m != nil && m.Role == schema.User
}

// IsAssistantMessage reports whether m is a non-nil assistant-role message.
func IsAssistantMessage(m *schema.Message) bool {
	return m != nil && m.Role == schema.Assistant
}

// IsToolMessage reports whether m is a non-nil tool-role message.
func IsToolMessage(m *schema.Message) bool {
	return m != nil && m.Role == schema.Tool
}

// CloneMessage returns a deep copy of m via JSON round-trip (safe for diverging Transcript vs Messages).
func CloneMessage(m *schema.Message) *schema.Message {
	if m == nil {
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var c schema.Message
	if err := json.Unmarshal(b, &c); err != nil {
		return m
	}
	return &c
}

// CloneMessages deep-copies each message pointer.
func CloneMessages(msgs []*schema.Message) []*schema.Message {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]*schema.Message, len(msgs))
	for i, m := range msgs {
		out[i] = CloneMessage(m)
	}
	return out
}
