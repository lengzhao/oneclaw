package wfexec

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/components/tool"

	tbuiltin "github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/lengzhao/oneclaw/workflow"
)

func handleWorkflowCommand(ctx context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	rtx := env.Runtime
	if rtx == nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: command: nil runtime")
	}
	cmd := strings.TrimSpace(in.Text)
	if cmd == "" {
		cmd = strings.TrimSpace(paramString(env.Node.Params, "command", "shell"))
	}
	if cmd == "" {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: command: empty command (set input/prompt or params.command)")
	}
	ws := strings.TrimSpace(rtx.EffectiveWorkspacePath())
	if ws == "" {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: command: empty workspace path")
	}
	timeoutSec := paramTimeoutSeconds(env.Node.Params)
	out, err := tbuiltin.RunExecInWorkspace(ctx, ws, tbuiltin.ExecInput{
		Command:        cmd,
		TimeoutSeconds: timeoutSec,
	})
	if err != nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: command: %w", err)
	}
	slog.InfoContext(ctx, "wfexec.workflow.command.done",
		"node", strings.TrimSpace(env.NodeID),
		"agent_type", runtimeAgentType(rtx),
		"correlation_id", strings.TrimSpace(rtx.CorrelationID),
		"output_chars", len(out),
	)
	text := strings.TrimSpace(out)
	data := map[string]any{}
	if paramBool(env.Node.Params, "parse_json_stdout") && text != "" && json.Valid([]byte(text)) {
		var parsed map[string]any
		if err := json.Unmarshal([]byte(text), &parsed); err == nil && parsed != nil {
			data = parsed
		}
	}
	return workflow.WorkflowNodeResult{Text: text, Data: data}, nil
}

func paramBool(params map[string]any, keys ...string) bool {
	if len(params) == 0 {
		return false
	}
	for _, k := range keys {
		v, ok := params[k]
		if !ok {
			continue
		}
		switch x := v.(type) {
		case bool:
			return x
		case string:
			s := strings.TrimSpace(strings.ToLower(x))
			return s == "true" || s == "1" || s == "yes"
		case int:
			return x != 0
		case float64:
			return x != 0
		}
	}
	return false
}

func handleWorkflowToolCall(ctx context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	rtx := env.Runtime
	if rtx == nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call: nil runtime")
	}
	if rtx.ToolRegistry == nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call: ToolRegistry not configured on runtime")
	}
	toolName := strings.TrimSpace(paramString(env.Node.Params, "tool", "name"))
	if toolName == "" {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call: params.tool (or params.name) required")
	}
	if workflowToolCallForbidden(toolName) {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call: tool %q is not allowed from workflow nodes", toolName)
	}
	args := strings.TrimSpace(in.Text)
	if args == "" {
		args = strings.TrimSpace(paramString(env.Node.Params, "arguments", "args"))
	}
	if args == "" {
		args = "{}"
	}
	if !json.Valid([]byte(args)) {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call: arguments must be JSON object text")
	}
	ts, err := rtx.ToolRegistry.FilterByNames([]string{toolName})
	if err != nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call: %w", err)
	}
	if len(ts) == 0 {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call: missing tool %q", toolName)
	}
	inv, ok := ts[0].(tool.InvokableTool)
	if !ok {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call: tool %q is not invokable", toolName)
	}
	slog.InfoContext(ctx, "wfexec.workflow.tool_call.start",
		"node", strings.TrimSpace(env.NodeID),
		"tool", toolName,
		"agent_type", runtimeAgentType(rtx),
		"correlation_id", strings.TrimSpace(rtx.CorrelationID),
	)
	out, err := inv.InvokableRun(ctx, args)
	if err != nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: tool_call %q: %w", toolName, err)
	}
	slog.InfoContext(ctx, "wfexec.workflow.tool_call.done",
		"node", strings.TrimSpace(env.NodeID),
		"tool", toolName,
		"output_chars", len(out),
	)
	return workflow.WorkflowNodeResult{Text: strings.TrimSpace(out)}, nil
}

func workflowToolCallForbidden(name string) bool {
	switch strings.TrimSpace(name) {
	case "run_agent":
		return true
	default:
		return false
	}
}

func paramString(params map[string]any, keys ...string) string {
	if len(params) == 0 {
		return ""
	}
	for _, k := range keys {
		if v, ok := params[k]; ok {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func paramTimeoutSeconds(params map[string]any) int {
	if len(params) == 0 {
		return 0
	}
	v, ok := params["timeout_seconds"]
	if !ok {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			return 0
		}
		var n int
		_, _ = fmt.Sscanf(s, "%d", &n)
		return n
	}
}
