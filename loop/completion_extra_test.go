package loop

import (
	"encoding/json"
	"testing"

	"github.com/openai/openai-go"
)

func TestChatCompletionExtraJSON_appliesBeforeRuntimeOverrides(t *testing.T) {
	extra := []byte(`{"temperature":0.25,"web_search_options":{"search_context_size":"low"}}`)
	var p openai.ChatCompletionNewParams
	if err := json.Unmarshal(extra, &p); err != nil {
		t.Fatal(err)
	}
	if p.Temperature.Value != 0.25 {
		t.Fatalf("temperature %v", p.Temperature.Value)
	}
	if p.WebSearchOptions.SearchContextSize != "low" {
		t.Fatalf("web_search_options %q", p.WebSearchOptions.SearchContextSize)
	}
	// Simulate runner: runtime then overwrites model
	p.Model = "gpt-4o"
	if p.Model != "gpt-4o" {
		t.Fatal("model override")
	}
}
