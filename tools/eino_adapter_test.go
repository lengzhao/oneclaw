package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type einoAdapterDummyTool struct {
	name string
}

func (t einoAdapterDummyTool) Name() string        { return t.name }
func (t einoAdapterDummyTool) Description() string { return "dummy" }
func (t einoAdapterDummyTool) Parameters() openai.FunctionParameters {
	return openai.FunctionParameters{
		"type": "object",
	}
}
func (t einoAdapterDummyTool) ConcurrencySafe() bool { return true }
func (t einoAdapterDummyTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	return t.name, nil
}

func TestRegistry_EinoBindings(t *testing.T) {
	r := NewRegistry()
	r.MustRegister(einoAdapterDummyTool{name: "z_tool"})
	r.MustRegister(einoAdapterDummyTool{name: "a_tool"})
	bindings := r.EinoBindings()
	if len(bindings) != 2 {
		t.Fatalf("bindings len: got %d want 2", len(bindings))
	}
	if bindings[0].Name != "a_tool" || bindings[1].Name != "z_tool" {
		t.Fatalf("bindings order: got %q, %q", bindings[0].Name, bindings[1].Name)
	}
	if len(bindings[0].ParametersJSON) == 0 || string(bindings[0].ParametersJSON) == "{}" {
		t.Fatalf("expected non-empty parameters json, got %q", string(bindings[0].ParametersJSON))
	}
}

