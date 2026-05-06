package wfexec

import (
	"context"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/engine"
	onetools "github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestHandleWorkflowCommand_execDeniedWithoutRuntimeConfig(t *testing.T) {
	defer config.PushRuntime(nil)
	tmp := t.TempDir()
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{WorkspacePath: tmp},
	}
	_, err := handleWorkflowCommand(context.Background(), NodeInput{Text: "echo hi"}, NodeEnv{
		Runtime: rtx,
		NodeID:  "c",
		Node:    workflow.Node{},
	})
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("expected exec denied, got %v", err)
	}
}

func TestHandleWorkflowCommand_runsWithExecPolicy(t *testing.T) {
	defer config.PushRuntime(nil)
	enabled := true
	config.PushRuntime(&config.File{
		Tools: map[string]config.ToolSwitch{
			config.BuiltinToolExec: {
				Enabled: &enabled,
				Allow:   []string{"*"},
			},
		},
	})
	tmp := t.TempDir()
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{WorkspacePath: tmp},
	}
	out, err := handleWorkflowCommand(context.Background(), NodeInput{Text: "echo ok"}, NodeEnv{
		Runtime: rtx,
		NodeID:  "c",
		Node:    workflow.Node{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.Text, "ok") {
		t.Fatalf("unexpected output: %q", out.Text)
	}
}

func TestHandleWorkflowToolCall_echo(t *testing.T) {
	tmp := t.TempDir()
	r := onetools.NewRegistry(tmp)
	echoTool, err := builtin.InferEcho()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Register(echoTool); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			WorkspacePath: tmp,
			ToolRegistry:  r,
		},
	}
	res, err := handleWorkflowToolCall(context.Background(), NodeInput{Text: `{"message":"hello"}`}, NodeEnv{
		Runtime: rtx,
		NodeID:  "tc",
		Node: workflow.Node{
			Params: map[string]any{"tool": builtin.NameEcho},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "hello" {
		t.Fatalf("got %q", res.Text)
	}
}

func TestHandleWorkflowToolCall_forbidsRunAgent(t *testing.T) {
	tmp := t.TempDir()
	r := onetools.NewRegistry(tmp)
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			WorkspacePath: tmp,
			ToolRegistry:  r,
		},
	}
	_, err := handleWorkflowToolCall(context.Background(), NodeInput{Text: `{}`}, NodeEnv{
		Runtime: rtx,
		Node:    workflow.Node{Params: map[string]any{"tool": "run_agent"}},
	})
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestHandleWorkflowToolCall_requiresToolName(t *testing.T) {
	tmp := t.TempDir()
	r := onetools.NewRegistry(tmp)
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			WorkspacePath: tmp,
			ToolRegistry:  r,
		},
	}
	_, err := handleWorkflowToolCall(context.Background(), NodeInput{Text: `{}`}, NodeEnv{
		Runtime: rtx,
		Node:    workflow.Node{Params: map[string]any{}},
	})
	if err == nil || !strings.Contains(err.Error(), "params.tool") {
		t.Fatalf("expected params error, got %v", err)
	}
}
