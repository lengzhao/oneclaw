package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type bashInput struct {
	Command    string `json:"command"`
	Timeout    int    `json:"timeout_sec"`
	Background bool   `json:"background"`
}

// backgroundStartTimeout caps how long we wait for the wrapper shell to fork and print $!.
const backgroundStartTimeout = 15 * time.Second

var bgPIDLine = regexp.MustCompile(`(?m)^ONECLAW_BG_PID=(\d+)\s*$`)

// BashTool runs a shell command in the working directory (unsafe; gate with CanUseTool in production).
type BashTool struct{}

func (BashTool) Name() string          { return "bash" }
func (BashTool) ConcurrencySafe() bool { return false }
func (BashTool) Description() string {
	return "Run a shell command via sh -c in the working directory (non-interactive: no TTY, stdin is /dev/null). " +
		"Do not run commands that need a password prompt; they will hang until timeout_sec. " +
		"Prefer: sudo -n, ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new, git with GIT_TERMINAL_PROMPT=0 (set automatically), npm --yes. " +
		"Optional timeout_sec (default 30, max 120; ignored when background is true). " +
		"When background is true, the command runs detached: returns immediately with PID and run log under cmd/.oneclaw/sessions/<session_id>/bash_log/<timestamp>/{pid}_run.log (stdout+stderr); host read_file as needed. Do not add a trailing & to command."
}

func (BashTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "Shell command passed to sh -c",
		},
		"timeout_sec": map[string]any{
			"type":        "integer",
			"description": "Timeout in seconds (default 30, max 120). Entire command is killed when exceeded; use for hung network or stuck prompts. Not used when background is true (start is capped at ~15s).",
		},
		"background": map[string]any{
			"type":        "boolean",
			"description": "If true, run detached under cmd/.oneclaw/sessions/<session_id>/bash_log/<timestamp>/{pid}_run.log; response includes pid and run_log path. Omit trailing & in command.",
		},
	}, []string{"command"})
}

func (BashTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	if strings.TrimSpace(in.Command) == "" {
		return "", fmt.Errorf("bash: empty command")
	}
	if in.Background {
		return bashExecuteBackground(ctx, in, tctx)
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

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func bashSessionPathSegment(id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "_"
	}
	id = strings.ReplaceAll(id, string(filepath.Separator), "_")
	id = strings.ReplaceAll(id, "..", "")
	id = strings.TrimSpace(id)
	if id == "" {
		return "_"
	}
	return id
}

// backgroundSessionDir is cmd/.oneclaw/sessions/<session_id>/bash_log/<unix_nanos>/ under cwd.
func backgroundSessionDir(cwd, sessionID string) (dir string, pendingLog string, err error) {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	sid := bashSessionPathSegment(sessionID)
	dir = filepath.Join(cwd, ".oneclaw", "sessions", sid, "bash_log", ts)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("bash: mkdir bash_log dir: %w", err)
	}
	pendingLog = filepath.Join(dir, "pending_run.log")
	return dir, pendingLog, nil
}

func bashExecuteBackground(ctx context.Context, in bashInput, tctx *toolctx.Context) (string, error) {
	_, pendingPath, err := backgroundSessionDir(tctx.CWD, tctx.SessionID)
	if err != nil {
		return "", err
	}
	qlog := shellSingleQuote(pendingPath)
	var script strings.Builder
	script.WriteString("{ ")
	script.WriteString(in.Command)
	script.WriteString("; } >>")
	script.WriteString(qlog)
	script.WriteString(` 2>&1 & printf 'ONECLAW_BG_PID=%s\n' "$!"`)

	base := tctx.Abort
	if base == nil {
		base = ctx
	}
	runCtx, cancel := context.WithTimeout(base, backgroundStartTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", script.String())
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
	combined := stdout.String()
	if stderr.Len() > 0 {
		if combined != "" {
			combined += "\n"
		}
		combined += stderr.String()
	}
	pid := ""
	if m := bgPIDLine.FindStringSubmatch(strings.TrimSpace(combined)); len(m) > 1 {
		pid = m[1]
	}

	runLogAbs := pendingPath
	if pid != "" {
		finalAbs := filepath.Join(filepath.Dir(pendingPath), pid+"_run.log")
		if rerr := os.Rename(pendingPath, finalAbs); rerr == nil {
			runLogAbs = finalAbs
		} else if _, statErr := os.Stat(pendingPath); statErr == nil {
			slog.Warn("bash.background.rename_log_failed", "from", pendingPath, "to", finalAbs, "err", rerr)
		}
	}

	relFromCWD, _ := filepath.Rel(tctx.CWD, runLogAbs)
	if relFromCWD == "" || strings.HasPrefix(relFromCWD, "..") {
		relFromCWD = runLogAbs
	}

	var b strings.Builder
	b.WriteString("background: true\n")
	b.WriteString("status: detached (session does not wait for this process)\n")
	if sid := strings.TrimSpace(tctx.SessionID); sid != "" {
		fmt.Fprintf(&b, "session_id: %s\n", sid)
	} else {
		b.WriteString("session_id: (empty; log dir uses \"_\" under cmd/.oneclaw/sessions/)\n")
	}
	if pid != "" {
		fmt.Fprintf(&b, "pid: %s\n", pid)
	} else {
		b.WriteString("pid: (unparsed; log may still be pending_run.log — see raw output)\n")
	}
	fmt.Fprintf(&b, "run_log: %s\n", runLogAbs)
	fmt.Fprintf(&b, "run_log_rel_cwd: %s\n", relFromCWD)
	if err != nil {
		fmt.Fprintf(&b, "wrapper_error: %v\n", err)
	}
	if strings.TrimSpace(combined) != "" {
		b.WriteString("raw_start_output:\n")
		b.WriteString(strings.TrimSpace(combined))
		b.WriteString("\n")
	}
	return b.String(), nil
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
