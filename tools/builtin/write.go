package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/pathutil"
	"github.com/openai/openai-go"
)

type writeInput struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

// WriteTool overwrites a file under the working directory (creates parent dirs).
type WriteTool struct{}

func (WriteTool) Name() string          { return "write_file" }
func (WriteTool) ConcurrencySafe() bool { return false }
func (WriteTool) Description() string {
	return "Write text content to a path under the working directory or under allowed memory roots (including ~/.oneclaw for user memory). Overwrites if the file exists. Creates parent directories."
}

func (WriteTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "File path relative to cwd",
		},
		"content": map[string]any{
			"type":        "string",
			"description": "Full file contents",
		},
	}, []string{"path", "content"})
}

func (WriteTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in writeInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	abs, err := pathutil.ResolveForSession(tctx.CWD, tctx.MemoryWriteRoots, in.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	content := []byte(in.Content)
	if err := os.WriteFile(abs, content, 0o644); err != nil {
		return "", err
	}
	tctx.SetCachedRead(abs, in.Content)
	return "ok", nil
}
