package tools

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/toolctx"
)

// Tool is a host-defined tool invoked by the model (OpenAI function calling).
type Tool interface {
	Name() string
	Description() string
	Parameters() openai.FunctionParameters
	ConcurrencySafe() bool
	Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (output string, err error)
}

// CanUseTool mirrors the permission gate before execution. Nil means allow all.
type CanUseTool func(ctx context.Context, name string, input json.RawMessage, tctx *toolctx.Context) (allow bool, reason string)
