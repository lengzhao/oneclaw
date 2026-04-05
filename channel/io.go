package channel

import (
	"context"

	"github.com/lengzhao/oneclaw/routing"
)

// Record is an outbound assistant/tool event (alias of routing.Record).
type Record = routing.Record

// Outbound kind constants for connectors that branch on Record.Kind without importing routing.
const (
	KindText = routing.KindText
	KindTool = routing.KindTool
	KindDone = routing.KindDone
)

// InboundTurn is one user turn from a connector into the runtime (no Engine/Sink types).
type InboundTurn struct {
	Text          string
	Ctx           context.Context // optional; if nil the router uses its parent ctx
	SessionKey    string
	UserID        string
	TenantID      string
	CorrelationID string
	// If non-nil, the router sends SubmitUser's result once (buffer the chan with size ≥1).
	Done chan<- error
}

// IO wires a connector to the channel runtime: push user turns on InboundChan, read assistant stream on OutboundChan.
type IO struct {
	InboundChan  chan<- InboundTurn
	OutboundChan <-chan Record
}
