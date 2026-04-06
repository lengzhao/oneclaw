package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type stubTool struct {
	name string
}

func (s stubTool) Name() string          { return s.name }
func (s stubTool) Description() string   { return s.name }
func (s stubTool) ConcurrencySafe() bool { return true }
func (s stubTool) Parameters() openai.FunctionParameters {
	return openai.FunctionParameters{"type": "object"}
}
func (s stubTool) Execute(context.Context, json.RawMessage, *toolctx.Context) (string, error) {
	return "", nil
}

func TestOpenAIToolsSortedByName(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(stubTool{name: "zebra"})
	_ = r.Register(stubTool{name: "alpha"})
	_ = r.Register(stubTool{name: "middle"})
	names := make([]string, 3)
	for i, p := range r.OpenAITools() {
		names[i] = p.Function.Name
	}
	if names[0] != "alpha" || names[1] != "middle" || names[2] != "zebra" {
		t.Fatalf("want alphabetical order, got %v", names)
	}
}
