package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool/utils"
)

type echoIn struct {
	Message string `json:"message" jsonschema:"description=Text to echo back"`
}

// RegisterBuiltins registers echo and read_file using registry.WorkspaceRoot as read boundary.
func RegisterBuiltins(r *Registry) error {
	echoT, err := utils.InferTool("echo", "Returns the input message unchanged.", func(ctx context.Context, in echoIn) (string, error) {
		return in.Message, nil
	})
	if err != nil {
		return err
	}
	if err := r.Register(echoT); err != nil {
		return err
	}

	root := strings.TrimSpace(r.WorkspaceRoot())
	if root == "" {
		return fmt.Errorf("tools: workspace root required for read_file")
	}
	base := filepath.Clean(root)

	type readIn struct {
		Path string `json:"path" jsonschema:"description=File path relative to workspace"`
	}
	readT, err := utils.InferTool("read_file", "Read a UTF-8 text file under the workspace.", func(ctx context.Context, in readIn) (string, error) {
		rel := filepath.ToSlash(strings.TrimSpace(in.Path))
		if rel == "" || strings.Contains(rel, "..") {
			return "", fmt.Errorf("invalid path")
		}
		full := filepath.Clean(filepath.Join(base, filepath.FromSlash(rel)))
		relOut, err := filepath.Rel(base, full)
		if err != nil || strings.HasPrefix(relOut, "..") {
			return "", fmt.Errorf("path escapes workspace")
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		return string(b), nil
	})
	if err != nil {
		return err
	}
	return r.Register(readT)
}
