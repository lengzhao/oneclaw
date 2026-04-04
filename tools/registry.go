package tools

import (
	"fmt"
	"slices"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared"
)

// Registry maps tool names to implementations and exposes OpenAI tool definitions.
type Registry struct {
	mu    sync.RWMutex
	items map[string]Tool
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

// OpenAITools builds ChatCompletionToolParam slice for ChatCompletionNewParams.
// Tool order is sorted by name so requests are stable across processes and runs.
func (r *Registry) OpenAITools() []openai.ChatCompletionToolParam {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.items))
	for name := range r.items {
		names = append(names, name)
	}
	slices.Sort(names)
	out := make([]openai.ChatCompletionToolParam, 0, len(names))
	for _, name := range names {
		t := r.items[name]
		out = append(out, openai.ChatCompletionToolParam{
			Function: shared.FunctionDefinitionParam{
				Name:        t.Name(),
				Description: openai.String(t.Description()),
				Parameters:  t.Parameters(),
			},
		})
	}
	return out
}
