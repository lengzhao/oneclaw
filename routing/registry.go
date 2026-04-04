package routing

import (
	"fmt"
	"sync"
)

// SinkRegistry resolves a Sink by channel source string (Inbound.Source).
type SinkRegistry interface {
	SinkFor(source string) (Sink, error)
}

// MapRegistry is a thread-safe map of source -> Sink.
type MapRegistry struct {
	mu sync.RWMutex
	m  map[string]Sink
}

// NewMapRegistry returns an empty registry.
func NewMapRegistry() *MapRegistry {
	return &MapRegistry{m: make(map[string]Sink)}
}

// Register binds source to s.
func (r *MapRegistry) Register(source string, s Sink) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.m == nil {
		r.m = make(map[string]Sink)
	}
	r.m[source] = s
}

// SinkFor implements SinkRegistry.
func (r *MapRegistry) SinkFor(source string) (Sink, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.m == nil {
		return nil, fmt.Errorf("routing: unknown source %q", source)
	}
	s, ok := r.m[source]
	if !ok {
		return nil, fmt.Errorf("routing: unknown source %q", source)
	}
	return s, nil
}
