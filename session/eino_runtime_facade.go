package session

import (
	"context"
	"fmt"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/tools"
)

// TurnRunner executes one user turn runtime loop (Eino ADK + tool bridge).
type TurnRunner interface {
	RunTurn(ctx context.Context, cfg loop.Config, in bus.InboundMessage) error
	Name() string
}

func defaultTurnRunner() TurnRunner {
	return newEinoTurnRunner()
}

type EinoExecutor interface {
	Execute(ctx context.Context, cfg loop.Config, in bus.InboundMessage, bindings []tools.EinoBinding) error
}

var newDefaultEinoExecutor = func() EinoExecutor {
	return newADKEinoExecutor()
}

func newEinoTurnRunner() TurnRunner {
	return einoTurnRunner{executor: newDefaultEinoExecutor()}
}

type einoTurnRunner struct {
	executor EinoExecutor
}

func (r einoTurnRunner) RunTurn(ctx context.Context, cfg loop.Config, in bus.InboundMessage) error {
	bindings, err := buildEinoToolBindings(cfg)
	if err != nil {
		return err
	}
	exec := r.executor
	if exec == nil {
		exec = newDefaultEinoExecutor()
	}
	return exec.Execute(ctx, cfg, in, bindings)
}

func (einoTurnRunner) Name() string {
	return "eino"
}

func buildEinoToolBindings(cfg loop.Config) ([]tools.EinoBinding, error) {
	if cfg.Registry == nil {
		return nil, fmt.Errorf("session: eino runtime requires tool registry")
	}
	return cfg.Registry.EinoBindings(), nil
}
