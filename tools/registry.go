package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sync"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

// Registry maps tool names to implementations and exposes stable tool descriptors.
type Registry struct {
	mu    sync.RWMutex
	items map[string]Tool
}

// Descriptor is a provider-neutral snapshot of a registered tool.
// It is the primary bridge type for adapting oneclaw tools to other runtimes.
type Descriptor struct {
	Name            string
	Description     string
	Parameters      openai.FunctionParameters
	ConcurrencySafe bool
	Execute         func(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error)
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{items: make(map[string]Tool)}
}

// Register adds a tool; duplicate names return an error.
func (r *Registry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := t.Name()
	if _, ok := r.items[name]; ok {
		return fmt.Errorf("tool %q already registered", name)
	}
	r.items[name] = t
	return nil
}

// MustRegister registers t or panics (for package init of builtins).
func (r *Registry) MustRegister(t Tool) {
	if err := r.Register(t); err != nil {
		panic(err)
	}
}

// Get returns a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.items[name]
	return t, ok
}

// ToolNames returns sorted registered tool names.
func (r *Registry) ToolNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.items))
	for name := range r.items {
		names = append(names, name)
	}
	slices.Sort(names)
	return names
}

// Descriptors returns stable-name-ordered provider-neutral tool descriptors.
func (r *Registry) Descriptors() []Descriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.items))
	for name := range r.items {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]Descriptor, 0, len(names))
	for _, name := range names {
		t := r.items[name]
		out = append(out, Descriptor{
			Name:            t.Name(),
			Description:     t.Description(),
			Parameters:      t.Parameters(),
			ConcurrencySafe: t.ConcurrencySafe(),
			Execute:         t.Execute,
		})
	}
	return out
}
