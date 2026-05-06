package wfexec

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestCompileEinoWorkflow_diamondJoin(t *testing.T) {
	ctx := context.Background()
	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "diamond",
		Nodes: map[string]workflow.Node{
			"a": {Use: "on_receive"},
			"b": {Use: "noop", DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"a"}},
			"d": {Use: "noop", DependsOn: []string{"b", "c"}},
		},
		End: "d",
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "hi"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	out, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "hi", Runtime: rtx})
	if err != nil {
		t.Fatal(err)
	}
	if out.Runtime != rtx {
		t.Fatal("expected same rtx pointer")
	}
}

func TestCompileEinoWorkflow_forkTwoSinks(t *testing.T) {
	ctx := context.Background()
	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "fork",
		Nodes: map[string]workflow.Node{
			"a": {Use: "on_receive"},
			"b": {Use: "noop", DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"a"}},
		},
		End: "b",
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "hi"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	out, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "hi", Runtime: rtx})
	if err != nil {
		t.Fatal(err)
	}
	if out.Runtime != rtx {
		t.Fatal("expected same rtx pointer")
	}
}

func TestCompileEinoWorkflow_linearStillWorks(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`workflow_spec_version: 2
id: linear
steps:
  - use: on_receive
  - use: noop
`)
	wf, err := workflow.ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "x"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "x", Runtime: rtx}); err != nil {
		t.Fatal(err)
	}
}

func TestCompileEinoWorkflow_ifReadsDynamicNodeDataAndRunsTrueBranch(t *testing.T) {
	ctx := context.Background()
	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "if-dynamic-data",
		Nodes: map[string]workflow.Node{
			"receive": {Use: "on_receive"},
			"stats":   {Use: "metrics_stub", DependsOn: []string{"receive"}},
			"gate": {
				Use:       "if",
				DependsOn: []string{"stats"},
				Params: map[string]any{
					"when_any": []any{
						map[string]any{"truthy": "$nodes.stats.tool_matched_read_skill"},
						map[string]any{"equals": []any{"$nodes.stats.tool_match_count_read_skill", 1}},
					},
				},
			},
			"next": {Use: "probe", DependsOn: []string{"gate"}},
		},
		End: "next",
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("metrics_stub", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		return workflow.WorkflowNodeResult{
			Data: map[string]any{
				"tool_matched_read_skill":      true,
				"tool_match_count_read_skill":  1,
				"distinct_tool_calls":          1,
				"skill_tree_tool_used":         true,
			},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	ranProbe := false
	if err := reg.Register("probe", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		ranProbe = true
		return workflow.WorkflowNodeResult{Text: "ok"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "x"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "x", Runtime: rtx}); err != nil {
		t.Fatal(err)
	}
	if !ranProbe {
		t.Fatal("expected true branch to run probe node")
	}
}

func TestCompileEinoWorkflow_skillGeneratorYAML_gateGTdoesNotError(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`workflow_spec_version: 2
id: skill_generator.turn
steps:
  - use: on_receive
  - id: skill_stats
    use: journal_tool_metrics
    input: $start.user_prompt
    params:
      match_tools:
        - read_skill
  - id: skill_gen_if
    use: if
    depends_on:
      - skill_stats
    params:
      when_any:
        - truthy: $nodes.skill_stats.tool_matched_read_skill
        - truthy: $nodes.skill_stats.skill_tree_tool_used
        - gt:
            - $nodes.skill_stats.distinct_tool_calls
            - 5
  - id: main
    use: noop
    depends_on:
      - skill_gen_if
  - id: respond
    use: noop
    depends_on:
      - main
end: respond
`)
	wf, err := workflow.ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("journal_tool_metrics", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		return workflow.WorkflowNodeResult{
			Text: `{"distinct_tool_calls":2}`,
			Data: map[string]any{
				"distinct_tool_calls":          2,
				"skill_tree_tool_used":         false,
				"tool_matched_read_skill":      false,
				"tool_match_count_read_skill":  0,
			},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("noop", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		return workflow.WorkflowNodeResult{Text: "ok"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "hi"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "hi", Runtime: rtx}); err != nil {
		t.Fatal(err)
	}
}

func TestCompileEinoWorkflow_ifReadsDynamicNodeDataAndSkipsFalseBranch(t *testing.T) {
	ctx := context.Background()
	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "if-dynamic-data-false",
		Nodes: map[string]workflow.Node{
			"receive": {Use: "on_receive"},
			"stats":   {Use: "metrics_stub", DependsOn: []string{"receive"}},
			"gate": {
				Use:       "if",
				DependsOn: []string{"stats"},
				Params: map[string]any{
					"when_any": []any{
						map[string]any{"truthy": "$nodes.stats.tool_matched_read_skill"},
					},
				},
			},
			"next": {
				Use:       "probe",
				DependsOn: []string{"gate"},
				Params: map[string]any{
					// Belt-and-suspenders: even if the DAG fires this node before branch metadata is merged,
					// skip the handler unless the if node recorded pass=true in Data.
					"require_truthy": "$nodes.gate.pass",
				},
			},
		},
		End: "gate",
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("metrics_stub", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		return workflow.WorkflowNodeResult{
			Data: map[string]any{
				"tool_matched_read_skill": false,
			},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	ranProbe := false
	if err := reg.Register("probe", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		ranProbe = true
		return workflow.WorkflowNodeResult{Text: "ok"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "x"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "x", Runtime: rtx}); err != nil {
		t.Fatal(err)
	}
	if ranProbe {
		t.Fatal("expected false branch to skip probe node")
	}
}

func TestCompileEinoWorkflow_skillGeneratorYAML_skipsMainWhenGateFalseAndRequireTruthy(t *testing.T) {
	ctx := context.Background()
	raw := []byte(`workflow_spec_version: 2
id: skill_generator.turn
steps:
  - use: on_receive
  - id: skill_stats
    use: journal_tool_metrics
    input: $start.user_prompt
    params:
      match_tools:
        - read_skill
  - id: skill_gen_if
    use: if
    depends_on:
      - skill_stats
    params:
      when_any:
        - truthy: $nodes.skill_stats.tool_matched_read_skill
        - truthy: $nodes.skill_stats.skill_tree_tool_used
        - gt:
            - $nodes.skill_stats.distinct_tool_calls
            - 5
  - id: main
    use: noop
    depends_on:
      - skill_gen_if
    params:
      require_truthy: $nodes.skill_gen_if.pass
  - id: respond
    use: noop
    input: $nodes.main
    depends_on:
      - main
end: respond
`)
	wf, err := workflow.ParseBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	var ranMain int32
	if err := reg.Register("journal_tool_metrics", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		return workflow.WorkflowNodeResult{
			Text: `{"distinct_tool_calls":0}`,
			Data: map[string]any{
				"distinct_tool_calls":          0,
				"skill_tree_tool_used":         false,
				"tool_matched_read_skill":      false,
				"tool_match_count_read_skill":  0,
			},
		}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("noop", func(_ context.Context, _ NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
		if env.NodeID == "main" {
			atomic.AddInt32(&ranMain, 1)
		}
		return workflow.WorkflowNodeResult{Text: "ok"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "hi"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "hi", Runtime: rtx}); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt32(&ranMain) != 0 {
		t.Fatalf("expected main skipped when gate false + require_truthy")
	}
}

