package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/workspace"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

// execInput matches picoclaw-style exec: primary field is command; cmd is an optional alias.
type execInput struct {
	Command    string `json:"command"`
	Cmd        string `json:"cmd"`
	Background bool   `json:"background"`
}

func (in *execInput) shellCommand() string {
	c := strings.TrimSpace(in.Command)
	if c != "" {
		return c
	}
	return strings.TrimSpace(in.Cmd)
}

func execShellChainHasRmGlobAll(cmdLine string) bool {
	for _, part := range shellChainSplitRE.Split(cmdLine, -1) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if rmGlobAllAtStartRE.MatchString(part) {
			return true
		}
	}
	return false
}

// execValidateSafety blocks commands that would kill the agent or its parent, or rm with a bare * in cwd.
func execValidateSafety(cmdLine string) error {
	cmdLine = strings.TrimSpace(cmdLine)
	if cmdLine == "" {
		return nil
	}
	if strings.Contains(cmdLine, "$PPID") || strings.Contains(cmdLine, "${PPID}") {
		return fmt.Errorf("exec: forbidden: command references $PPID (cannot target parent process)")
	}
	if execShellChainHasRmGlobAll(cmdLine) {
		return fmt.Errorf("exec: forbidden: rm with bare * (delete-all in cwd); use explicit paths or narrower globs (e.g. *.go)")
	}
	if !killWordRE.MatchString(cmdLine) {
		return nil
	}
	ppid := os.Getppid()
	pid := os.Getpid()
	if ppid > 0 {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(strconv.Itoa(ppid)) + `\b`)
		if re.MatchString(cmdLine) {
			return fmt.Errorf("exec: forbidden: kill targets parent process (pid %d)", ppid)
		}
	}
	if pid > 0 {
		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(strconv.Itoa(pid)) + `\b`)
		if re.MatchString(cmdLine) {
			return fmt.Errorf("exec: forbidden: kill targets the agent process (pid %d)", pid)
		}
	}
	return nil
}

// backgroundStartTimeout caps how long we wait for the wrapper shell to start the job and write the pid file.
const backgroundStartTimeout = 15 * time.Second

// foregroundSyncWait is how long the tool waits for a foreground command before returning partial info (timed_out + run_log + pid). Overridable in tests.
var foregroundSyncWait = 30 * time.Second

// rmGlobAllAtStartRE matches an rm whose last path arg is a bare cwd-wide glob, only at the
// start of a shell chain segment (split on ; && ||). So "echo rm *" is allowed; "cd x && rm *" is not.
var rmGlobAllAtStartRE = regexp.MustCompile(`(?i)^rm(?:\s+[^\s*]+)*\s+(?:\./)?\*(\s|$|[;&|])`)

// killWordRE matches the kill(1) command name only (word boundaries; not substrings like "skill").
var killWordRE = regexp.MustCompile(`(?i)\bkill\b`)

var shellChainSplitRE = regexp.MustCompile(`\s*(?:&&|\|\||;)\s*`)

// ExecTool runs a shell command in the working directory (unsafe; gate with CanUseTool in production).
// Naming and primary parameter align with picoclaw's exec tool; this build does not implement
// picoclaw's session actions (poll/read/write/kill on sessionId) or PTY — only run + optional background / wait-timeout detach.
type ExecTool struct{}

func (ExecTool) Name() string          { return "exec" }
func (ExecTool) ConcurrencySafe() bool { return false }
func (ExecTool) Description() string {
	return "Execute shell commands via sh -c in the working directory (non-interactive: no TTY, stdin is /dev/null). " +
		"Same role as picoclaw `exec` run action; no poll/read/write/kill sessions here. " +
		"Do not run commands that need a password prompt. " +
		"Prefer: sudo -n, ssh -o BatchMode=yes -o StrictHostKeyChecking=accept-new, git with GIT_TERMINAL_PROMPT=0 (set automatically), npm --yes. " +
		"Foreground: the tool waits up to 30 seconds; if the command finishes in time, full output is returned. If not, returns partial info (timed_out, pid, run_log) while the process may still run — you choose follow-up (e.g. read_file on run.log, or background: true). " +
		"pid in that case is cmd.Process.Pid (the sh -c child). " +
		"Run log path: session runtime `exec_log/<timestamp>/run.log` (stdout+stderr only; cwd is the session workspace). Background: pid is the detached job ($!). Do not add a trailing & to command."
}

func (ExecTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"command": map[string]any{
			"type":        "string",
			"description": "Shell command passed to sh -c (preferred; same as picoclaw exec run)",
		},
		"cmd": map[string]any{
			"type":        "string",
			"description": "Optional alias for command when the model sends cmd instead of command",
		},
		"background": map[string]any{
			"type":        "boolean",
			"description": "If true, run detached under the session runtime `exec_log/<timestamp>/run.log`; response includes pid and run_log path. Omit trailing & in command.",
		},
	}, []string{"command"})
}

func (ExecTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in execInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	cmdLine := in.shellCommand()
	if cmdLine == "" {
		return "", fmt.Errorf("exec: empty command (set command, or cmd as alias)")
	}
	in.Command = cmdLine
	if err := execValidateSafety(cmdLine); err != nil {
		return "", err
	}
	if in.Background {
		return execExecuteBackground(ctx, in, tctx)
	}
	return execExecuteForegroundWaitOrDetach(ctx, in, tctx)
}

// execBackgroundScript runs userCmd with stdout+stderr appended to log, backgrounds it, writes job pid to pidFile (not into the run log).
func execBackgroundScript(userCmd, quotedLogPath, quotedPidFile string) string {
	var b strings.Builder
	b.WriteString("{ ")
	b.WriteString(userCmd)
	b.WriteString("; } >>")
	b.WriteString(quotedLogPath)
	b.WriteString(` 2>&1 & pid=$!; printf '%s\n' "$pid" >`)
	b.WriteString(quotedPidFile)
	return b.String()
}

func execExecuteForegroundWaitOrDetach(ctx context.Context, in execInput, tctx *toolctx.Context) (string, error) {
	base := tctx.Abort
	if base == nil {
		base = ctx
	}

	_, runLogPath, err := execSessionDir(tctx.CWD, tctx.SessionID, tctx.WorkspaceFlat, tctx.InstructionRoot)
	if err != nil {
		return "", err
	}
	logF, err := os.OpenFile(runLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", fmt.Errorf("exec: open run log: %w", err)
	}

	cmd := exec.Command("sh", "-c", in.Command)
	cmd.Dir = tctx.CWD
	cmd.Env = nonInteractiveShellEnv()
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		_ = logF.Close()
		return "", fmt.Errorf("exec: open stdin: %w", err)
	}
	defer stdin.Close()
	cmd.Stdin = stdin
	cmd.Stdout = logF
	cmd.Stderr = logF

	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		return "", fmt.Errorf("exec: start: %w", err)
	}
	pid := strconv.Itoa(cmd.Process.Pid)
	_ = logF.Close()
	runLogAbs := runLogPath

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	timer := time.NewTimer(foregroundSyncWait)
	defer timer.Stop()

	select {
	case <-base.Done():
		_ = cmd.Process.Kill()
		_ = <-waitErr
		return "", base.Err()
	case err := <-waitErr:
		if !timer.Stop() {
			<-timer.C
		}
		logBytes, rerr := os.ReadFile(runLogAbs)
		if rerr != nil {
			return "", fmt.Errorf("exec: read run log: %w", rerr)
		}
		return formatForegroundLogOutput(logBytes, err), nil
	case <-timer.C:
		go func() { _ = <-waitErr }()
		return execDetachedResponse(tctx, runLogAbs, pid, nil, "wait_timeout"), nil
	}
}

func formatForegroundLogOutput(logBytes []byte, waitErr error) string {
	var b strings.Builder
	if waitErr != nil {
		// Lead with failure so tool-trace / notify previews (prefix-limited) still surface the reason.
		fmt.Fprintf(&b, "exec_failed: %v\n", waitErr)
	}
	b.Write(logBytes)
	out := b.String()
	if strings.TrimSpace(out) == "" {
		return "(no output)"
	}
	return out
}

// execDetachedResponse formats pid/run_log metadata; kind is "background" or "wait_timeout".
func execDetachedResponse(tctx *toolctx.Context, runLogAbs, pid string, wrapperErr error, kind string) string {
	relFromCWD, _ := filepath.Rel(tctx.CWD, runLogAbs)
	if relFromCWD == "" || strings.HasPrefix(relFromCWD, "..") {
		relFromCWD = runLogAbs
	}
	var b strings.Builder
	if wrapperErr != nil {
		fmt.Fprintf(&b, "exec_wrapper_error: %v\n", wrapperErr)
	}
	switch kind {
	case "wait_timeout":
		b.WriteString("timed_out: true\n")
		fmt.Fprintf(&b, "status: waited %s; command may still be running — use run_log / read_file; decide next step\n", foregroundSyncWait)
	default:
		b.WriteString("background: true\n")
		b.WriteString("status: detached (session does not wait for this process)\n")
	}
	if sid := strings.TrimSpace(tctx.SessionID); sid != "" {
		fmt.Fprintf(&b, "session_id: %s\n", sid)
	} else {
		b.WriteString("session_id: (empty)\n")
	}
	if pid != "" {
		fmt.Fprintf(&b, "pid: %s\n", pid)
	} else {
		b.WriteString("pid: (unparsed)\n")
	}
	fmt.Fprintf(&b, "run_log: %s\n", runLogAbs)
	fmt.Fprintf(&b, "run_log_rel_cwd: %s\n", relFromCWD)
	return b.String()
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

// execSessionDir creates exec_log/<unix_ts>/ under the session runtime dir (see memory.JoinSessionWorkspaceWithInstruction).
func execSessionDir(cwd, sessionID string, workspaceFlat bool, instructionRoot string) (dir string, runLogPath string, err error) {
	_ = sessionID
	ts := fmt.Sprintf("%d", time.Now().Unix())
	dir = workspace.JoinSessionWorkspaceWithInstruction(cwd, instructionRoot, workspaceFlat, "exec_log", ts)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", fmt.Errorf("exec: mkdir exec_log dir: %w", err)
	}
	runLogPath = filepath.Join(dir, "run.log")
	return dir, runLogPath, nil
}

func execExecuteBackground(ctx context.Context, in execInput, tctx *toolctx.Context) (string, error) {
	_, runLogPath, err := execSessionDir(tctx.CWD, tctx.SessionID, tctx.WorkspaceFlat, tctx.InstructionRoot)
	if err != nil {
		return "", err
	}
	pidFile := filepath.Join(filepath.Dir(runLogPath), "pending_child.pid")
	script := execBackgroundScript(in.Command, shellSingleQuote(runLogPath), shellSingleQuote(pidFile))

	base := tctx.Abort
	if base == nil {
		base = ctx
	}
	runCtx, cancel := context.WithTimeout(base, backgroundStartTimeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, "sh", "-c", script)
	cmd.Dir = tctx.CWD
	cmd.Env = nonInteractiveShellEnv()
	stdin, err := os.Open(os.DevNull)
	if err != nil {
		return "", fmt.Errorf("exec: open stdin: %w", err)
	}
	defer stdin.Close()
	cmd.Stdin = stdin
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard

	err = cmd.Run()
	pid := ""
	if b, rerr := os.ReadFile(pidFile); rerr == nil {
		pid = strings.TrimSpace(string(b))
		if _, perr := strconv.Atoi(pid); perr != nil {
			pid = ""
		}
	}
	_ = os.Remove(pidFile)

	return execDetachedResponse(tctx, runLogPath, pid, err, "background"), nil
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
