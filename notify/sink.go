package notify

import (
	"context"
	"log/slog"
)

// Sink receives lifecycle events synchronously. Implementations must return quickly;
// heavy work should be delegated outside the hot path.
type Sink interface {
	Emit(ctx context.Context, ev Event) error
}

// EmitSafe calls sink.Emit with panic recovery; errors are logged and ignored for the agent turn.
func EmitSafe(sink Sink, ctx context.Context, ev Event) {
	if sink == nil {
		return
	}
	defer func() {
		if r := recover(); r != nil {
			slog.Error("notify.emit_panic", "event", ev.Event, "recover", r)
		}
	}()
	if err := sink.Emit(ctx, ev); err != nil {
		slog.Warn("notify.emit_failed", "event", ev.Event, "err", err)
	}
}

// Multi fans out each event to every non-nil sink in order.
// session.Engine.Notify is a Multi by default; use (*Multi).Register or Engine.RegisterNotify to add sinks.
//
// EmitSafe ignores Multi.Emit's error return and logs one warn if Emit returns an error.
type Multi []Sink

func (m Multi) Emit(ctx context.Context, ev Event) error {
	var first error
	for _, s := range m {
		if s == nil {
			continue
		}
		if err := s.Emit(ctx, ev); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// Register appends non-nil sinks to this fan-out list (e.g. eng.Notify.Register(sink)).
func (m *Multi) Register(sinks ...Sink) {
	if m == nil {
		return
	}
	for _, s := range sinks {
		if s != nil {
			*m = append(*m, s)
		}
	}
}
