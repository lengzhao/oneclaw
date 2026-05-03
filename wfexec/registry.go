package wfexec

import (
	"fmt"

	"github.com/lengzhao/oneclaw/engine"
)

// Handler runs one node instance.
type Handler func(rtx *engine.RuntimeContext) error

// Registry maps workflow use → implementation.
type Registry struct {
	h map[string]Handler
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{h: map[string]Handler{}}
}

// Register adds or replaces a handler for use.
func (r *Registry) Register(use string, h Handler) error {
	if r == nil {
		return fmt.Errorf("wfexec: nil registry")
	}
	if use == "" || h == nil {
		return fmt.Errorf("wfexec: invalid register")
	}
	r.h[use] = h
	return nil
}

// Lookup returns handler or nil.
func (r *Registry) Lookup(use string) Handler {
	if r == nil {
		return nil
	}
	return r.h[use]
}
