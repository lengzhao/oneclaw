package routing

// defaultRegistry is populated by channel-specific init hooks (e.g. CLI).
var defaultRegistry = NewMapRegistry()

// DefaultRegistry returns the process-wide sink registry.
func DefaultRegistry() SinkRegistry {
	return defaultRegistry
}

// RegisterDefaultSink registers a sink on DefaultRegistry (e.g. HTTP server at startup).
func RegisterDefaultSink(source string, s Sink) {
	defaultRegistry.Register(source, s)
}
