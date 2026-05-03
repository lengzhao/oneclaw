package turnhub

import (
	"context"
	"time"

	clawbridge "github.com/lengzhao/clawbridge"
)

// HubOption configures [NewHub].
type HubOption func(*Hub)

// WithMaxBuf sets the per-session inbound channel capacity (default 256). Values ≤ 0 are ignored.
func WithMaxBuf(n int) HubOption {
	return func(h *Hub) {
		if n > 0 {
			h.maxBuf = n
		}
	}
}

// WithTurnTimeout wraps each turn in [context.WithTimeout] derived from the hub parent ctx.
// Zero means no per-turn deadline (processor receives the hub root context only).
func WithTurnTimeout(d time.Duration) HubOption {
	return func(h *Hub) {
		h.turnTimeout = d
	}
}

// WithOnDropped is invoked synchronously when the session mailbox drops the oldest queued job
// because the channel buffer is full; use it to notify the user (e.g. clawbridge Reply).
func WithOnDropped(f func(ctx context.Context, dropped clawbridge.InboundMessage) error) HubOption {
	return func(h *Hub) {
		h.onDropped = f
	}
}
