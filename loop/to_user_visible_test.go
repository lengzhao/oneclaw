package loop_test

import (
	"encoding/json"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/openai/openai-go"
)

func TestToUserVisibleMessages_dropsInjectionsAndTools(t *testing.T) {
	t.Parallel()
	agentMd := "Codebase and user instructions are shown below. Follow them; they override defaults.\n\n" +
		"<system-reminder>\n# agentMd\n\nx\n</system-reminder>"
	payload, err := json.Marshal(map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": agentMd},
			map[string]any{"role": "user", "content": "<inbound-context>\nsource: web\n</inbound-context>"},
			map[string]any{"role": "user", "content": "Attachment: relevant_memories\n\nsnippet"},
			map[string]any{"role": "user", "content": "[oneclaw:compact_boundary ts=2020-01-01T00:00:00Z]\nEarlier\n\n[/oneclaw:compact_boundary]\n"},
			map[string]any{"role": "user", "content": "人类可见问题"},
			map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
				"id": "c1", "type": "function", "function": map[string]any{"name": "bash", "arguments": "{}"},
			}}},
			map[string]any{"role": "tool", "tool_call_id": "c1", "content": "shell out"},
			map[string]any{"role": "assistant", "content": "最终答复"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := loop.UnmarshalMessages(payload)
	if err != nil {
		t.Fatal(err)
	}
	out := loop.ToUserVisibleMessages(msgs)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if loop.UserMessageText(out[0]) != "人类可见问题" {
		t.Fatalf("user: %q", loop.UserMessageText(out[0]))
	}
	if loop.AssistantParamText(out[1]) != "最终答复" {
		t.Fatalf("assistant: %q", loop.AssistantParamText(out[1]))
	}
}

func TestToUserVisibleMessages_stripsToolCallsFromMixedAssistant(t *testing.T) {
	t.Parallel()
	payload, err := json.Marshal(map[string]any{
		"messages": []any{
			map[string]any{"role": "user", "content": "run it"},
			map[string]any{
				"role":    "assistant",
				"content": "好的",
				"tool_calls": []any{map[string]any{
					"id": "c1", "type": "function", "function": map[string]any{"name": "bash", "arguments": "{}"},
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	msgs, err := loop.UnmarshalMessages(payload)
	if err != nil {
		t.Fatal(err)
	}
	out := loop.ToUserVisibleMessages(msgs)
	if len(out) != 2 {
		t.Fatalf("len=%d want 2", len(out))
	}
	if loop.AssistantParamText(out[1]) != "好的" {
		t.Fatalf("got %q", loop.AssistantParamText(out[1]))
	}
	if out[1].OfAssistant != nil && len(out[1].OfAssistant.ToolCalls) != 0 {
		t.Fatal("expected tool_calls stripped")
	}
}

func TestToUserVisibleMessages_keepsNormalUserWithAgentMdMention(t *testing.T) {
	t.Parallel()
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("讨论 # agentMd 这个标题可以吗"),
	}
	out := loop.ToUserVisibleMessages(msgs)
	if len(out) != 1 {
		t.Fatalf("len=%d want 1", len(out))
	}
}
