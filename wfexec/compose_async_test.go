package wfexec

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestCompilePhase3Workflow_asyncContinuesBeforeHandlerDone(t *testing.T) {
	workflow.Phase3Uses["stall"] = struct{}{}
	t.Cleanup(func() { delete(workflow.Phase3Uses, "stall") })

	ctx := context.Background()
	entered := make(chan struct{}, 1)
	reg := NewRegistry()
	if err := RegisterPhase3Builtins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("stall", func(*engine.RuntimeContext) error {
		entered <- struct{}{}
		time.Sleep(400 * time.Millisecond)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	wf := &workflow.Workflow{
		SpecVersion: 1,
		ID:          "async-chain",
		Graph: workflow.Graph{
			Entry: "a",
			Nodes: map[string]workflow.Node{
				"a": {Use: "on_receive"},
				"b": {Use: "stall", Async: true},
				"c": {Use: "noop"},
			},
			Edges: []workflow.Edge{
				{From: "a", To: "b"},
				{From: "b", To: "c"},
			},
		},
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}

	run, err := CompilePhase3Workflow(ctx, wf, reg)
	if err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{UserPrompt: "hi"}

	errCh := make(chan error, 1)
	go func() {
		_, err := run.Invoke(ctx, rtx)
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

func TestCompilePhase3Workflow_asyncRecordsFailure(t *testing.T) {
	workflow.Phase3Uses["boom"] = struct{}{}
	t.Cleanup(func() { delete(workflow.Phase3Uses, "boom") })

	ctx := context.Background()
	reg := NewRegistry()
	if err := RegisterPhase3Builtins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("boom", func(*engine.RuntimeContext) error {
		return context.Canceled
	}); err != nil {
		t.Fatal(err)
	}

	wf := &workflow.Workflow{
		SpecVersion: 1,
		ID:          "async-fail",
		Graph: workflow.Graph{
			Entry: "a",
			Nodes: map[string]workflow.Node{
				"a": {Use: "on_receive"},
				"b": {Use: "boom", Async: true},
				"c": {Use: "noop"},
			},
			Edges: []workflow.Edge{
				{From: "a", To: "b"},
				{From: "b", To: "c"},
			},
		},
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}
	run, err := CompilePhase3Workflow(ctx, wf, reg)
	if err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{UserPrompt: "hi"}
	if _, err := run.Invoke(ctx, rtx); err != nil {
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

func TestCompilePhase3Workflow_asyncEffectiveUserPromptUsesForkSnapshot(t *testing.T) {
	for _, u := range []string{"mutate_prompt", "effective_prompt_check"} {
		workflow.Phase3Uses[u] = struct{}{}
	}
	t.Cleanup(func() {
		delete(workflow.Phase3Uses, "mutate_prompt")
		delete(workflow.Phase3Uses, "effective_prompt_check")
	})

	var effectiveSeen string
	done := make(chan struct{})
	ctx := context.Background()
	reg := NewRegistry()
	if err := RegisterPhase3Builtins(reg); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("mutate_prompt", func(rtx *engine.RuntimeContext) error {
		rtx.UserPrompt = "mutated-after-async-scheduled"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := reg.Register("effective_prompt_check", func(rtx *engine.RuntimeContext) error {
		effectiveSeen = rtx.EffectiveUserPrompt()
		close(done)
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	wf := &workflow.Workflow{
		SpecVersion: 1,
		ID:          "async-snapshot-prompt",
		Graph: workflow.Graph{
			Entry: "a",
			Nodes: map[string]workflow.Node{
				"a": {Use: "on_receive"},
				"b": {Use: "effective_prompt_check", Async: true},
				"c": {Use: "mutate_prompt"},
			},
			Edges: []workflow.Edge{
				{From: "a", To: "b"},
				{From: "b", To: "c"},
			},
		},
	}
	if err := workflow.Validate(wf); err != nil {
		t.Fatal(err)
	}
	run, err := CompilePhase3Workflow(ctx, wf, reg)
	if err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{UserPrompt: "original"}

	if _, err := run.Invoke(ctx, rtx); err != nil {
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
