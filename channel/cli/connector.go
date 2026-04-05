package cli

import (
	"context"
	"sync"

	"github.com/lengzhao/oneclaw/channel"
)

// RegistryName is Spec.Key for the default CLI connector.
const RegistryName = "cli"

// Terminal is the stdin/stdout REPL connector (no session/routing imports).
type Terminal struct{}

// New builds a terminal connector.
func New(channel.ConnectorConfig) (channel.Connector, error) {
	return &Terminal{}, nil
}

func (t *Terminal) Name() string { return RegistryName }

// Run reads stdin and writes assistant output from IO.OutboundChan.
func (t *Terminal) Run(ctx context.Context, io channel.IO) error {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		printOutbound(ctx, io)
	}()
	defer wg.Wait()
	return stdinLoop(ctx, io)
}
