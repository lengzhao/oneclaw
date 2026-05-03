package builtin

import (
	"context"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

type echoIn struct {
	Message string `json:"message" jsonschema:"description=Text to echo back"`
}

// InferEcho builds the echo builtin (no workspace).
func InferEcho() (tool.InvokableTool, error) {
	return utils.InferTool(NameEcho, "Returns the input message unchanged.", func(ctx context.Context, in echoIn) (string, error) {
		return in.Message, nil
	})
}
