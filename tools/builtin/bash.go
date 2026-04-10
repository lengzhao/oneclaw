package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout_sec"`
}

// BashTool runs a shell command in the working directory (unsafe; gate with CanUseTool in production).
type BashTool struct{}

func (BashTool) Name() string          { return "bash" }
func (BashTool) ConcurrencySafe() bool { return false }
func (BashTool) Description() string {
	return "Run a shell command via sh -c in the working directory (non-interactive: no TTY, stdin is /dev/null). " +
		"Do not run commands that need a password prompt; they will hang until timeout_sec. " +
		"Prefer: sudo -n, ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new, git with GIT_TERMINAL_PROMPT=0 (set automatically), npm --yes. " +
		"Optional timeout_sec (default 30, max 120)."
}

func (BashTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "Shell command passed to sh -c",
		},
		"timeout_sec": map[string]any{
			"type":        "integer",
			"description": "Timeout in seconds (default 30, max 120). Entire command is killed when exceeded; use for hung network or stuck prompts.",
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
	cmd.Env = nonInteractiveShellEnv()
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return "", fmt.Errorf("bash: open stdin: %w", err)
	}
	defer stdin.Close()
	cmd.Stdin = stdin
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
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

// nonInteractiveShellEnv returns the process environment with variables that make common CLIs
// fail fast or skip TTY prompts instead of blocking the agent (stdin is already /dev/null).
func nonInteractiveShellEnv() []string {
	base := os.Environ()
	m := make(map[string]string, len(base)+8)
	for _, e := range base {
		i := strings.IndexByte(e, '=')
		if i <= 0 {
			continue
		}
		m[e[:i]] = e[i+1:]
	}
	// Generic: many scripts skip prompts when CI is set.
	m["CI"] = "true"
	m["DEBIAN_FRONTEND"] = "noninteractive"
	m["GIT_TERMINAL_PROMPT"] = "0"
	m["NPM_CONFIG_YES"] = "true"
	// curl: fail on HTTP errors without hanging on slow reads; does not fix password prompts.
	if _, ok := m["CURL_NO_SIGNAL"]; !ok {
		m["CURL_NO_SIGNAL"] = "1"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}
