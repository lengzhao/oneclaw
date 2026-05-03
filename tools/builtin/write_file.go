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

type writeFileIn struct {
	Path    string `json:"path" jsonschema:"description=File path relative to workspace (UTF-8 text)"`
	Content string `json:"content" jsonschema:"description=Full file contents to write"`
}

// InferWriteFile builds the write_file builtin bound to workspaceRoot.
func InferWriteFile(workspaceRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameWriteFile)
	}
	return utils.InferTool(NameWriteFile, "Create or overwrite a UTF-8 text file under the workspace. Creates parent directories as needed.",
		func(ctx context.Context, in writeFileIn) (string, error) {
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
			if err := os.WriteFile(full, []byte(in.Content), 0o644); err != nil {
				return "", err
			}
			return fmt.Sprintf("wrote %d bytes to %s", len(in.Content), filepath.ToSlash(strings.TrimSpace(in.Path))), nil
		})
}
