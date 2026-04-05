package channel

// defaultRegistry receives RegisterDefault from channel/* init hooks.
var defaultRegistry = NewRegistry()

// DefaultRegistry returns the process-wide channel registry.
func DefaultRegistry() *Registry {
	return defaultRegistry
}

// RegisterDefault registers a Spec on DefaultRegistry (Sink + Channel constructor). Typically called from init in channel/*.
func RegisterDefault(s Spec) {
	defaultRegistry.RegisterSpec(s)
}
