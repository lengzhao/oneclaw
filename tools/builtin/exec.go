package builtin

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/lengzhao/oneclaw/config"
)

const (
	maxExecOutputBytes = 256 * 1024
	defaultExecTimeout = 30 * time.Second
	maxExecTimeoutSec  = 120
)

type execIn struct {
	Command string `json:"command" jsonschema:"description=Shell command; working directory is workspace root. Requires config tools.exec enabled with matching allow prefix."`
	// TimeoutSeconds optional wall-clock limit (default 30, max 120).
	TimeoutSeconds int `json:"timeout_seconds,omitempty" jsonschema:"description=Timeout seconds (default 30, max 120)"`
}

// InferExec builds the exec builtin (cwd = workspace). Policy from [config.ExecCommandAllowedFromRuntime].
func InferExec(workspaceRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameExec)
	}
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}
	return utils.InferTool(NameExec, "Run a shell command with workspace as cwd. Default disabled; set tools.exec.enabled and tools.exec.allow prefix list in config (deny substrings optional). Output capped.",
		func(ctx context.Context, in execIn) (string, error) {
			cmd := strings.TrimSpace(in.Command)
			if cmd == "" {
				return "", fmt.Errorf("command required")
			}
			if !config.ExecCommandAllowedFromRuntime(cmd) {
				return "", fmt.Errorf("exec denied (configure tools.exec: enabled, allow prefixes, PushRuntime)")
			}
			timeout := defaultExecTimeout
			if in.TimeoutSeconds > 0 {
				sec := in.TimeoutSeconds
				if sec > maxExecTimeoutSec {
					sec = maxExecTimeoutSec
				}
				timeout = time.Duration(sec) * time.Second
			}
			runCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			var c *exec.Cmd
			if runtime.GOOS == "windows" {
				c = exec.CommandContext(runCtx, "cmd", "/C", cmd)
			} else {
				c = exec.CommandContext(runCtx, "sh", "-c", cmd)
			}
			c.Dir = absRoot
			out, err := c.CombinedOutput()
			if len(out) > maxExecOutputBytes {
				out = append([]byte{}, out[:maxExecOutputBytes]...)
				out = append(out, []byte("\n[truncated]\n")...)
			}
			s := string(out)
			if err != nil {
				if strings.TrimSpace(s) == "" {
					return "", fmt.Errorf("exec: %w", err)
				}
				return fmt.Sprintf("%v\n%s", err, s), nil
			}
			if strings.TrimSpace(s) == "" {
				return "(no output)", nil
			}
			return s, nil
		})
}
