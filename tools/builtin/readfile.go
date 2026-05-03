package builtin

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/lengzhao/oneclaw/tools/workspace"
)

type readFileIn struct {
	Path string `json:"path" jsonschema:"description=File path relative to workspace"`
}

// InferReadFile builds the read_file builtin bound to workspaceRoot.
func InferReadFile(workspaceRoot string) (tool.InvokableTool, error) {
	root := workspaceRoot
	if root == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameReadFile)
	}
	return utils.InferTool(NameReadFile, "Read a UTF-8 text file under the workspace.", func(ctx context.Context, in readFileIn) (string, error) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if strings.TrimSpace(in.Path) == "" {
			return "", fmt.Errorf("path required")
		}
		full, err := workspace.ResolveUnderWorkspace(root, in.Path)
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		return string(b), nil
	})
}
