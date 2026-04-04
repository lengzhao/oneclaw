package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/toolctx"
)

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout_sec"`
}

// BashTool runs a shell command in the working directory (unsafe; gate with CanUseTool in production).
type BashTool struct{}

func (BashTool) Name() string        { return "bash" }
func (BashTool) ConcurrencySafe() bool { return false }
func (BashTool) Description() string {
	return "Run a shell command via sh -c in the working directory. Optional timeout_sec (default 30, max 120)."
}

func (BashTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "Shell command passed to sh -c",
		},
		"timeout_sec": map[string]any{
			"type":        "integer",
			"description": "Timeout in seconds (default 30, max 120)",
		},
	}, []string{"command"})
}

func (BashTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	sec := in.Timeout
	if sec <= 0 {
		sec = 30
	}
	if sec > 120 {
		sec = 120
	}
	base := tctx.Abort
	if base == nil {
		base = ctx
	}
	runCtx, cancel := context.WithTimeout(base, time.Duration(sec)*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "sh", "-c", in.Command)
	cmd.Dir = tctx.CWD
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	out := stdout.String()
	if stderr.Len() > 0 {
		if out != "" {
			out += "\n"
		}
		out += "stderr:\n" + stderr.String()
	}
	if err != nil {
		if out != "" {
			out += "\n"
		}
		out += fmt.Sprintf("error: %v", err)
	}
	if out == "" {
		out = "(no output)"
	}
	return out, nil
}
