package session

import (
	"context"

	"github.com/lengzhao/clawbridge"
	"github.com/lengzhao/clawbridge/bus"
)

// publishOutbound enqueues msg on [Engine.Bridge]'s bus. If Bridge is nil, returns [clawbridge.ErrNotInitialized] (same as an uninitialized process default).
func (e *Engine) publishOutbound(ctx context.Context, msg *bus.OutboundMessage) error {
	if e == nil || e.Bridge == nil {
		return clawbridge.ErrNotInitialized
	}
	return e.Bridge.Bus().PublishOutbound(ctx, msg)
}

// updateInboundStatus calls [Bridge.UpdateStatus]. If Bridge is nil, returns [clawbridge.ErrNotInitialized].
func (e *Engine) updateInboundStatus(ctx context.Context, in *bus.InboundMessage, state clawbridge.UpdateStatusState, metadata map[string]string) error {
	if e == nil || e.Bridge == nil {
		return clawbridge.ErrNotInitialized
	}
	return e.Bridge.UpdateStatus(ctx, in, state, metadata)
}
