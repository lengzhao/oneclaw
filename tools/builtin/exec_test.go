package builtin

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/toolctx"
)

func TestMain(m *testing.M) {
	// Foreground sync wait is 30s in production; shorten for tests that assert timed_out.
	foregroundSyncWait = 750 * time.Millisecond
	os.Exit(m.Run())
}

func TestNonInteractiveShellEnv_setsExpectedKeys(t *testing.T) {
	env := nonInteractiveShellEnv()
	m := map[string]string{}
	for _, e := range env {
		i := strings.IndexByte(e, '=')
		if i <= 0 {
			continue
		}
		m[e[:i]] = e[i+1:]
	}
	if m["CI"] != "true" {
		t.Fatalf("CI: got %q", m["CI"])
	}
	if m["GIT_TERMINAL_PROMPT"] != "0" {
		t.Fatalf("GIT_TERMINAL_PROMPT: got %q", m["GIT_TERMINAL_PROMPT"])
	}
	if m["DEBIAN_FRONTEND"] != "noninteractive" {
		t.Fatalf("DEBIAN_FRONTEND: got %q", m["DEBIAN_FRONTEND"])
	}
}

func TestExecTool_Execute_nonInteractiveEnvInChild(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	var et ExecTool
	raw, err := json.Marshal(map[string]any{"command": "printf 'CI=%s GIT=%s' \"$CI\" \"$GIT_TERMINAL_PROMPT\""})
	if err != nil {
		t.Fatal(err)
	}
	out, err := et.Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CI=true") || !strings.Contains(out, "GIT=0") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestExecTool_Execute_cmdAlias(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	var et ExecTool
	raw, err := json.Marshal(map[string]any{"cmd": "echo alias_ok"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := et.Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "alias_ok") {
		t.Fatalf("expected alias_ok, got %q", out)
	}
}

func TestExecTool_Execute_stdinIsDevNull(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	var et ExecTool
	// Interactive-style read would hang if stdin were inherited from a TTY; /dev/null gives immediate EOF.
	raw, err := json.Marshal(map[string]any{"command": "read -r line || true; echo done"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := et.Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "done") {
		t.Fatalf("expected done, got %q", out)
	}
}

func TestExecTool_Execute_backgroundReturnsQuickly(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	tctx.SessionID = "02a6242438ccb577d1faf4af"
	var et ExecTool
	raw, err := json.Marshal(map[string]any{
		"command":    "sleep 120",
		"background": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	out, err := et.Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("background start took too long: %s", time.Since(start))
	}
	if !strings.Contains(out, "background: true") || !strings.Contains(out, "pid:") {
		t.Fatalf("expected background marker and pid line, got %q", out)
	}
	if !strings.Contains(out, "run_log:") || !strings.Contains(out, ".oneclaw/sessions/") {
		t.Fatalf("expected run_log under .oneclaw/sessions, got %q", out)
	}
	if !strings.Contains(out, "exec_log") {
		t.Fatalf("expected exec_log in path, got %q", out)
	}
	if !strings.Contains(out, "02a6242438ccb577d1faf4af") {
		t.Fatalf("expected session_id in path/output, got %q", out)
	}
	// Parse PID and reap so the test process tree stays clean.
	rest := out
	for {
		i := strings.Index(rest, "pid:")
		if i < 0 {
			break
		}
		rest = strings.TrimSpace(rest[i+4:])
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			break
		}
		if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
			break
		}
	}
}

func TestExecTool_Execute_waitTimeoutReturnsDetachInfo(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	tctx.SessionID = "wait-timeout-sess"
	var et ExecTool
	raw, err := json.Marshal(map[string]any{
		"command": "sleep 120",
	})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	out, err := et.Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatalf("expected return before long sleep, took %s", time.Since(start))
	}
	if !strings.Contains(out, "timed_out: true") || !strings.Contains(out, "pid:") {
		t.Fatalf("expected timed_out and pid, got %q", out)
	}
	if !strings.Contains(out, "run_log:") || !strings.Contains(out, ".oneclaw/sessions/") {
		t.Fatalf("expected run_log path, got %q", out)
	}
	// Reap detached sleep so the test process tree stays clean.
	rest := out
	for {
		i := strings.Index(rest, "pid:")
		if i < 0 {
			break
		}
		rest = strings.TrimSpace(rest[i+4:])
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			break
		}
		if pid, err := strconv.Atoi(fields[0]); err == nil && pid > 0 {
			if p, err := os.FindProcess(pid); err == nil {
				_ = p.Kill()
			}
			break
		}
	}
}
