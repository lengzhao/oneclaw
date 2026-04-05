package routing

import (
	"context"
	"errors"
	"fmt"
)

// ErrUseRegistrySink is returned by SinkFactory.NewSink to fall back to SinkRegistry.SinkFor(in.Source).
var ErrUseRegistrySink = errors.New("routing: use registry sink")

// SinkFactory builds a per-turn Sink from inbound metadata (e.g. bind Slack thread_ts).
// When NewSink returns (nil, ErrUseRegistrySink), ResolveTurnSink uses the static registry.
type SinkFactory interface {
	NewSink(ctx context.Context, in Inbound) (Sink, error)
}

// ResolveTurnSink picks the sink for one SubmitUser turn.
// Priority: SinkFactory (unless it returns ErrUseRegistrySink), then SinkRegistry.
// Both may be nil; then (nil, nil) means no outbound emitter.
func ResolveTurnSink(ctx context.Context, reg SinkRegistry, factory SinkFactory, in Inbound) (Sink, error) {
	if factory != nil {
		s, err := factory.NewSink(ctx, in)
		if err != nil {
			if errors.Is(err, ErrUseRegistrySink) {
				return sinkFromRegistry(reg, in.Source)
			}
			return nil, fmt.Errorf("routing: sink factory: %w", err)
		}
		if s != nil {
			return s, nil
		}
	}
	return sinkFromRegistry(reg, in.Source)
}

func sinkFromRegistry(reg SinkRegistry, source string) (Sink, error) {
	if reg == nil {
		return nil, nil
	}
	return reg.SinkFor(source)
}
