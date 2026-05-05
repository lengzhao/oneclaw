package tools

import (
	"context"
	"errors"
	"fmt"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

// WrapInvokableSoftErrors returns a tool that never fails InvokableRun: failures become a normal tool payload so the LLM can retry or recover instead of aborting the workflow graph.
func WrapInvokableSoftErrors(inner tool.InvokableTool) tool.InvokableTool {
	if inner == nil {
		return nil
	}
	return softErrTool{inner: inner}
}

type softErrTool struct {
	inner tool.InvokableTool
}

func (w softErrTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return w.inner.Info(ctx)
}

func (w softErrTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	out, err := w.inner.InvokableRun(ctx, argumentsInJSON, opts...)
	if ctx.Err() != nil {
		return "", ctx.Err()
	}
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return "", err
		}
		return fmt.Sprintf("[tool_error] %v", err), nil
	}
	return out, nil
}
