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

type editFileIn struct {
	Path    string `json:"path" jsonschema:"description=File path relative to workspace (UTF-8 text)"`
	OldText string `json:"old_text" jsonschema:"description=Exact substring to replace; must occur exactly once (JSON: \\n newline)"`
	NewText string `json:"new_text" jsonschema:"description=Replacement text (may be empty to delete old_text; JSON: \\n newline)"`
}

// InferEditFile builds the edit_file builtin (PicoClaw-style exact replace) bound to workspaceRoot.
func InferEditFile(workspaceRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameEditFile)
	}
	return utils.InferTool(NameEditFile, "Edit a UTF-8 file under the workspace by replacing one exact occurrence of old_text with new_text. Safer than overwriting whole files.",
		func(ctx context.Context, in editFileIn) (string, error) {
			if strings.TrimSpace(in.Path) == "" {
				return "", fmt.Errorf("path required")
			}
			if in.OldText == "" {
				return "", fmt.Errorf("old_text required")
			}
			full, err := workspace.ResolveUnderWorkspace(root, in.Path)
			if err != nil {
				return "", err
			}
			st, err := os.Stat(full)
			if err != nil {
				return "", err
			}
			if st.IsDir() {
				return "", fmt.Errorf("path is a directory")
			}
			raw, err := os.ReadFile(full)
			if err != nil {
				return "", err
			}
			if len(raw) > workspace.MaxWorkspaceWriteBytes {
				return "", fmt.Errorf("file exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
			}
			body := string(raw)
			n := strings.Count(body, in.OldText)
			if n == 0 {
				return "", fmt.Errorf("old_text not found")
			}
			if n > 1 {
				return "", fmt.Errorf("old_text matches %d times; must match exactly once", n)
			}
			out := strings.Replace(body, in.OldText, in.NewText, 1)
			if len(out) > workspace.MaxWorkspaceWriteBytes {
				return "", fmt.Errorf("result exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
			}
			if err := os.WriteFile(full, []byte(out), 0o644); err != nil {
				return "", err
			}
			return fmt.Sprintf("edited %s (%d -> %d bytes)", filepath.ToSlash(strings.TrimSpace(in.Path)), len(body), len(out)), nil
		})
}
