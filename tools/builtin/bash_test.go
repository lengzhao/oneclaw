package builtin

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/toolctx"
)

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

func TestBashTool_Execute_nonInteractiveEnvInChild(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	var bt BashTool
	raw, err := json.Marshal(map[string]any{"command": "printf 'CI=%s GIT=%s' \"$CI\" \"$GIT_TERMINAL_PROMPT\""})
	if err != nil {
		t.Fatal(err)
	}
	out, err := bt.Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "CI=true") || !strings.Contains(out, "GIT=0") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestBashTool_Execute_stdinIsDevNull(t *testing.T) {
	cwd := t.TempDir()
	tctx := toolctx.New(cwd, context.Background())
	var bt BashTool
	// Interactive-style read would hang if stdin were inherited from a TTY; /dev/null gives immediate EOF.
	raw, err := json.Marshal(map[string]any{"command": "read -r line || true; echo done"})
	if err != nil {
		t.Fatal(err)
	}
	out, err := bt.Execute(context.Background(), raw, tctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "done") {
		t.Fatalf("expected done, got %q", out)
	}
}
