package loop

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go"
)

func TestAssistantParamFromCompletion_preservesReasoningAndCustomExtras(t *testing.T) {
	raw := `{"role":"assistant","content":"","reasoning_content":"stepwise thought","vendor_flag":true,"tool_calls":[{"id":"c1","type":"function","function":{"name":"x","arguments":"{}"}}]}`
	var msg openai.ChatCompletionMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		t.Fatal(err)
	}
	u := assistantParamFromCompletion(msg)
	if u.OfAssistant == nil {
		t.Fatal("expected assistant variant")
	}
	ex := u.OfAssistant.ExtraFields()
	if ex == nil {
		t.Fatal("expected extra fields")
	}
	if s, _ := ex["reasoning_content"].(string); s != "stepwise thought" {
		t.Fatalf("reasoning_content = %v (%T)", ex["reasoning_content"], ex["reasoning_content"])
	}
	if ex["vendor_flag"] != true {
		t.Fatalf("vendor_flag = %v", ex["vendor_flag"])
	}
	b, err := json.Marshal(u)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out["reasoning_content"] != "stepwise thought" {
		t.Fatalf("marshaled reasoning_content = %v", out["reasoning_content"])
	}
}

func TestApplyOutboundAssistantExtensionFields_fillsMissingReasoning(t *testing.T) {
	msgs := []openai.ChatCompletionMessageParamUnion{
		openai.UserMessage("hi"),
		openai.ChatCompletionMessageParamUnion{
			OfAssistant: &openai.ChatCompletionAssistantMessageParam{
				ToolCalls: []openai.ChatCompletionMessageToolCallParam{
					{ID: "c1", Type: "function", Function: openai.ChatCompletionMessageToolCallFunctionParam{Name: "n", Arguments: "{}"}},
				},
			},
		},
	}
	ApplyOutboundAssistantExtensionFields(msgs)
	asst := msgs[1].OfAssistant
	if asst == nil {
		t.Fatal("assistant missing")
	}
	ex := asst.ExtraFields()
	if ex == nil || ex["reasoning_content"] != "" {
		t.Fatalf("extras = %#v", ex)
	}
}
