package loop

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
)

type fakeTool struct {
	name string
	safe bool
}

func (f fakeTool) Name() string          { return f.name }
func (f fakeTool) Description() string   { return f.name }
func (f fakeTool) ConcurrencySafe() bool { return f.safe }
func (f fakeTool) Parameters() openai.FunctionParameters {
	return openai.FunctionParameters{"type": "object"}
}
func (f fakeTool) Execute(context.Context, json.RawMessage, *toolctx.Context) (string, error) {
	return "", nil
}

func TestPartitionToolCalls(t *testing.T) {
	reg := tools.NewRegistry()
	reg.MustRegister(fakeTool{name: "read_a", safe: true})
	reg.MustRegister(fakeTool{name: "read_b", safe: true})
	reg.MustRegister(fakeTool{name: "write", safe: false})

	calls := []toolCallInv{
		{Name: "read_a"},
		{Name: "read_b"},
		{Name: "write"},
		{Name: "read_a"},
	}
	batches := partitionToolCalls(calls, reg)
	if len(batches) != 3 {
		t.Fatalf("want 3 batches, got %d", len(batches))
	}
	if len(batches[0]) != 2 || len(batches[1]) != 1 || len(batches[2]) != 1 {
		t.Fatalf("unexpected batch sizes: %#v", batches)
	}
}
