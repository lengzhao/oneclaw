package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/lengzhao/oneclaw/tools/workspace"
)

type appendFileIn struct {
	Path    string `json:"path" jsonschema:"description=File path relative to workspace"`
	Content string `json:"content" jsonschema:"description=Text to append (UTF-8)"`
}

// InferAppendFile builds the append_file builtin bound to workspaceRoot.
func InferAppendFile(workspaceRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameAppendFile)
	}
	return utils.InferTool(NameAppendFile, "Append UTF-8 text to a file under the workspace. Creates the file if missing.",
		func(ctx context.Context, in appendFileIn) (string, error) {
			if strings.TrimSpace(in.Path) == "" {
				return "", fmt.Errorf("path required")
			}
			if len(in.Content) > workspace.MaxWorkspaceWriteBytes {
				return "", fmt.Errorf("content exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
			}
			full, err := workspace.ResolveUnderWorkspace(root, in.Path)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", err
			}
			f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return "", err
			}
			defer f.Close()
			n, err := f.WriteString(in.Content)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("appended %d bytes to %s", n, filepath.ToSlash(strings.TrimSpace(in.Path))), nil
		})
}
