package wfexec

import (
	"context"
	"fmt"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/workflow"
)

// NodeInput is the explicit data contract for one workflow node execution.
type NodeInput struct {
	Text string
	Data map[string]any
}

// NodeEnv provides runtime services for node handlers.
type NodeEnv struct {
	Runtime *engine.RuntimeContext
	NodeID  string
	Node    workflow.Node
}

// Handler runs one node instance and returns a structured result.
type Handler func(ctx context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error)

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
