package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/pathutil"
)

const maxReadBytes = 256 * 1024

type readInput struct {
	Path string `json:"path"`
}

// ReadTool reads a text file under the session working directory.
type ReadTool struct{}

func (ReadTool) Name() string        { return "read_file" }
func (ReadTool) ConcurrencySafe() bool { return true }
func (ReadTool) Description() string {
	return "Read file contents from a path under the working directory or under ~/.oneclaw / .oneclaw memory roots. Text only; capped at 256KiB."
}

func (ReadTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "File path relative to cwd or absolute under cwd",
		},
	}, []string{"path"})
}

func (ReadTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in readInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	abs, err := pathutil.ResolveForSession(tctx.CWD, tctx.MemoryWriteRoots, in.Path)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if st.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file")
	}
	if s, ok := tctx.GetCachedRead(abs); ok {
		return s, nil
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	if len(b) > maxReadBytes {
		return string(b[:maxReadBytes]) + "\n\n[truncated: file exceeds 256KiB]", nil
	}
	out := string(b)
	tctx.SetCachedRead(abs, out)
	return out, nil
}
