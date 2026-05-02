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

func TestEinoBindingsSortedByName(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(stubTool{name: "zebra"})
	_ = r.Register(stubTool{name: "alpha"})
	_ = r.Register(stubTool{name: "middle"})
	b := r.EinoBindings()
	if len(b) != 3 {
		t.Fatalf("len=%d", len(b))
	}
	if b[0].Name != "alpha" || b[1].Name != "middle" || b[2].Name != "zebra" {
		t.Fatalf("want alphabetical order, got %v, %v, %v", b[0].Name, b[1].Name, b[2].Name)
	}
}
