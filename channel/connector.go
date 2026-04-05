package channel

import "context"

// Connector is a channel implementation: only terminal (or SDK) I/O via IO.
// The channel package connects IO to session.Engine and routing.Sink.
type Connector interface {
	Name() string
	Run(ctx context.Context, io IO) error
}
