package routing

import "context"

// Sink receives outbound records (CLI, HTTP SSE, noop, etc.).
type Sink interface {
	Emit(ctx context.Context, r Record) error
}

// NoopSink drops all events.
type NoopSink struct{}

func (NoopSink) Emit(context.Context, Record) error { return nil }
