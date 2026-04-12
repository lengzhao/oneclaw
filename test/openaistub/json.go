package openaistub

import "encoding/json"

// CompletionStop returns a minimal chat.completion JSON (non-stream) ending with assistant text.
func CompletionStop(model, content string) []byte {
	if model == "" {
		model = "gpt-4o"
	}
	m := map[string]any{
		"id":      "chatcmpl-stub",
		"object":  "chat.completion",
		"created": int64(1700000000),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"finish_reason": "stop",
				"logprobs":      nil,
				"message": map[string]any{
					"role":    "assistant",
					"content": content,
					"refusal": "",
				},
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     1,
			"completion_tokens": 1,
			"total_tokens":      2,
		},
	}
	b, _ := json.Marshal(m)
	return b
}

// CompletionToolCalls returns a chat.completion that requests tools (finish_reason tool_calls).
func CompletionToolCalls(model string, toolCalls []map[string]any) []byte {
	if model == "" {
		model = "gpt-4o"
	}
	m := map[string]any{
		"id":      "chatcmpl-stub-tool",
		"object":  "chat.completion",
		"created": int64(1700000000),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"finish_reason": "tool_calls",
				"logprobs":      nil,
				"message": map[string]any{
					"role":       "assistant",
					"content":    "",
					"refusal":    "",
					"tool_calls": toolCalls,
				},
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     1,
			"completion_tokens": 1,
			"total_tokens":      2,
		},
	}
	b, _ := json.Marshal(m)
	return b
}

// CompletionToolCallsEmptyFinishReason is like CompletionToolCalls but sets finish_reason to "".
// Some OpenAI-compatible gateways omit or clear finish_reason when returning tool_calls.
func CompletionToolCallsEmptyFinishReason(model string, toolCalls []map[string]any) []byte {
	if model == "" {
		model = "gpt-4o"
	}
	m := map[string]any{
		"id":      "chatcmpl-stub-tool",
		"object":  "chat.completion",
		"created": int64(1700000000),
		"model":   model,
		"choices": []any{
			map[string]any{
				"index":         0,
				"finish_reason": "",
				"logprobs":      nil,
				"message": map[string]any{
					"role":       "assistant",
					"content":    "",
					"refusal":    "",
					"tool_calls": toolCalls,
				},
			},
		},
		"usage": map[string]any{
			"prompt_tokens":     1,
			"completion_tokens": 1,
			"total_tokens":      2,
		},
	}
	b, _ := json.Marshal(m)
	return b
}

// ToolCall builds one OpenAI-style tool_calls[] element (type function).
func ToolCall(id, name, argumentsJSON string) map[string]any {
	return map[string]any{
		"id":   id,
		"type": "function",
		"function": map[string]any{
			"name":      name,
			"arguments": argumentsJSON,
		},
	}
}
