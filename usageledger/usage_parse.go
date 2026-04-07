package usageledger

import (
	"encoding/json"
	"strings"
)

// ParseCostUSDFromUsageJSON extracts a dollar amount when the provider embeds it in the usage object
// (OpenAI 官方 chat.completion 的 usage 通常只有 token 计数；部分兼容网关会加 cost / cost_usd 等字段).
func ParseCostUSDFromUsageJSON(usageJSON string) (usd float64, ok bool) {
	usageJSON = strings.TrimSpace(usageJSON)
	if usageJSON == "" || usageJSON == "null" {
		return 0, false
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(usageJSON), &m); err != nil {
		return 0, false
	}
	// Order: more specific keys first.
	for _, key := range []string{
		"cost_usd", "total_cost_usd", "openai_cost", "estimated_cost_usd",
		"cost", "total_cost", "estimated_cost", "amount", "usd",
	} {
		raw, exists := m[key]
		if !exists {
			continue
		}
		var f float64
		if err := json.Unmarshal(raw, &f); err == nil && f >= 0 {
			return f, true
		}
	}
	return 0, false
}
