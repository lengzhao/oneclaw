package wfexec

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestCompileEinoWorkflow_asyncContinuesBeforeHandlerDone(t *testing.T) {
	workflow.AllowedUses["stall"] = struct{}{}
	t.Cleanup(func() { delete(workflow.AllowedUses, "stall") })

	ctx := context.Background()
	entered := make(chan struct{}, 1)
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("stall", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		entered <- struct{}{}
		time.Sleep(400 * time.Millisecond)
		return workflow.WorkflowNodeResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "async-chain",
		Nodes: map[string]workflow.Node{
			"a": {Use: "on_receive"},
			"b": {Use: "stall", Async: true, DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"b"}},
		},
		End: "c",
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}

	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "hi"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		_, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "hi", Runtime: rtx})
		errCh <- err
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Invoke did not return quickly while async handler still running")
	}

	select {
	case <-entered:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("async stall handler never started")
	}

	if finished, _ := rtx.AsyncHandlerFinished("b"); finished {
		t.Fatal("async node should still be running (stall sleeping)")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		finished, handlerErr := rtx.AsyncHandlerFinished("b")
		if finished {
			if handlerErr != nil {
				t.Fatalf("handlerErr=%v", handlerErr)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("async handler never recorded completion")
}

func TestCompileEinoWorkflow_asyncRecordsFailure(t *testing.T) {
	workflow.AllowedUses["boom"] = struct{}{}
	t.Cleanup(func() { delete(workflow.AllowedUses, "boom") })

	ctx := context.Background()
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("boom", func(_ context.Context, _ NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
		return workflow.WorkflowNodeResult{}, context.Canceled
	}); err != nil {
		t.Fatal(err)
	}

	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "async-fail",
		Nodes: map[string]workflow.Node{
			"a": {Use: "on_receive"},
			"b": {Use: "boom", Async: true, DependsOn: []string{"a"}},
			"c": {Use: "noop", DependsOn: []string{"b"}},
		},
		End: "c",
	}
	if err := workflow.Validate(wf); err != nil {
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

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		finished, herr := rtx.AsyncHandlerFinished("b")
		if finished {
			if herr == nil {
				t.Fatal("expected handler error")
			}
			if !errors.Is(herr, context.Canceled) {
				t.Fatalf("unexpected err: %v", herr)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("async handler never finished")
}

func TestCompileEinoWorkflow_asyncEffectiveUserPromptUsesForkSnapshot(t *testing.T) {
	for _, u := range []string{"mutate_prompt", "effective_prompt_check"} {
		workflow.AllowedUses[u] = struct{}{}
	}
	t.Cleanup(func() {
		delete(workflow.AllowedUses, "mutate_prompt")
		delete(workflow.AllowedUses, "effective_prompt_check")
	})

	var effectiveSeen string
	done := make(chan struct{})
	ctx := context.Background()
	reg := NewRegistry()
	if err := RegisterBuiltins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("mutate_prompt", func(_ context.Context, _ NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
		rtx := env.Runtime
		rtx.UserPrompt = "mutated-after-async-scheduled"
		return workflow.WorkflowNodeResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("effective_prompt_check", func(_ context.Context, _ NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
		rtx := env.Runtime
		effectiveSeen = rtx.EffectiveUserPrompt()
		close(done)
		return workflow.WorkflowNodeResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}

	wf := &workflow.Workflow{
		SpecVersion: 2,
		ID:          "async-snapshot-prompt",
		Nodes: map[string]workflow.Node{
			"a": {Use: "on_receive"},
			"b": {Use: "effective_prompt_check", Async: true, DependsOn: []string{"a"}},
			"c": {Use: "mutate_prompt", DependsOn: []string{"b"}},
		},
		End: "c",
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{TurnInputs: engine.TurnInputs{UserPrompt: "original"}}
	run, err := CompileEinoWorkflow(ctx, wf, reg, rtx)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := run.Invoke(ctx, TurnWorkflowInput{UserPrompt: "original", Runtime: rtx}); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("async handler did not run")
	}

	if effectiveSeen != "original" {
		t.Fatalf("EffectiveUserPrompt in async handler: got %q want %q (rtx.UserPrompt=%q)", effectiveSeen, "original", rtx.UserPrompt)
	}
	if rtx.UserPrompt != "mutated-after-async-scheduled" {
		t.Fatalf("live UserPrompt: got %q", rtx.UserPrompt)
	}
}
