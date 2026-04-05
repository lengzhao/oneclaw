package openaistub

import (
	"encoding/json"
	"strings"
)

// ChatRequestUserTextConcat extracts and joins string/text content from all messages with role "user"
// in a chat.completions request body (for e2e assertions).
func ChatRequestUserTextConcat(body []byte) (string, error) {
	var req struct {
		Messages []struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, m := range req.Messages {
		if m.Role != "user" {
			continue
		}
		if s := decodeMessageContent(m.Content); s != "" {
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(s)
		}
	}
	return b.String(), nil
}

func decodeMessageContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	if raw[0] == '"' {
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s
		}
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err == nil {
		var b strings.Builder
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				if b.Len() > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return ""
}
