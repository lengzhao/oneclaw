package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cloudwego/eino/components/tool"
	"github.com/lengzhao/oneclaw/config"
)

func TestExecTool_deniedWithoutRuntime(t *testing.T) {
	ctx := context.Background()
	ws := t.TempDir()
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolExec}); err != nil {
		t.Fatal(err)
	}
	ts, err := r.FilterByNames([]string{ToolExec})
	if err != nil {
		t.Fatal(err)
	}
	_, err = ts[0].(tool.InvokableTool).InvokableRun(ctx, `{"command":"echo hi"}`)
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("want denied error, got %v", err)
	}
}

func TestExecTool_withRuntimePolicy(t *testing.T) {
	prev := config.Runtime()
	t.Cleanup(func() {
		if prev == nil {
			config.PushRuntime(nil)
			return
		}
		config.PushRuntime(prev.Config)
	})

	en := true
	cfg := &config.File{
		Tools: map[string]config.ToolSwitch{
			config.BuiltinToolExec: {Enabled: &en, Allow: []string{"echo "}},
		},
	}
	config.ApplyDefaults(cfg)
	config.PushRuntime(cfg)

	ctx := context.Background()
	ws := t.TempDir()
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolExec}); err != nil {
		t.Fatal(err)
	}
	ts, err := r.FilterByNames([]string{ToolExec})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"command": "echo ok"})
	out, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(args))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "ok") {
		t.Fatalf("got %q", out)
	}
}
