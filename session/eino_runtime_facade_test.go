package session

import (
	"context"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/tools"
)

func TestDefaultTurnRunner_Name(t *testing.T) {
	r := defaultTurnRunner()
	if r == nil {
		t.Fatal("defaultTurnRunner() returned nil")
	}
	if got := r.Name(); got != "eino" {
		t.Fatalf("default runner: got %q want eino", got)
	}
}

func TestNewEngine_SetsTurnRunner(t *testing.T) {
	e := NewEngine(t.TempDir(), tools.NewRegistry())
	if e.TurnRunner == nil {
		t.Fatal("NewEngine should set default TurnRunner")
	}
	if e.TurnRunner.Name() != "eino" {
		t.Fatalf("default TurnRunner: %q", e.TurnRunner.Name())
	}
}

func TestBuildEinoToolBindings(t *testing.T) {
	if _, err := buildEinoToolBindings(loop.Config{}); err == nil {
		t.Fatal("expected error when registry is nil")
	}
	cfg := loop.Config{Registry: tools.NewRegistry()}
	b, err := buildEinoToolBindings(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(b) != 0 {
		t.Fatalf("expected empty bindings, got %d", len(b))
	}
}

type recorderEinoExecutor struct {
	called bool
}

func (r *recorderEinoExecutor) Execute(ctx context.Context, cfg loop.Config, in bus.InboundMessage, bindings []tools.EinoBinding) error {
	r.called = true
	return nil
}

func TestEinoTurnRunner_UsesExecutor(t *testing.T) {
	rec := &recorderEinoExecutor{}
	runner := einoTurnRunner{executor: rec}
	err := runner.RunTurn(context.Background(), loop.Config{Registry: tools.NewRegistry()}, bus.InboundMessage{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rec.called {
		t.Fatal("expected custom executor to be called")
	}
}

func TestNewEinoTurnRunner_UsesDefaultFactory(t *testing.T) {
	orig := newDefaultEinoExecutor
	t.Cleanup(func() { newDefaultEinoExecutor = orig })

	rec := &recorderEinoExecutor{}
	newDefaultEinoExecutor = func() EinoExecutor { return rec }

	runner := newEinoTurnRunner()
	if runner == nil {
		t.Fatal("expected non-nil runner")
	}
	if runner.Name() != "eino" {
		t.Fatalf("runner name: %q", runner.Name())
	}
	if err := runner.RunTurn(context.Background(), loop.Config{Registry: tools.NewRegistry()}, bus.InboundMessage{}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !rec.called {
		t.Fatal("expected default executor factory result to be used")
	}
}
