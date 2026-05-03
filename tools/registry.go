// Package tools provides the tool registry and builtins (FR-FLOW-04 filter hooks).
package tools

import (
	"context"
	"fmt"
	"sync"

	"github.com/cloudwego/eino/components/tool"
)

// Registry holds BaseTool instances by unique name.
type Registry struct {
	mu        sync.RWMutex
	order     []string
	byName    map[string]tool.BaseTool
	workspace string // optional cwd anchor for path tools
}

// NewRegistry builds an empty registry. workspaceRoot may be empty if unused.
func NewRegistry(workspaceRoot string) *Registry {
	return &Registry{
		byName:    make(map[string]tool.BaseTool),
		workspace: workspaceRoot,
	}
}

// WorkspaceRoot returns the directory bound at construction (for read_file).
func (r *Registry) WorkspaceRoot() string {
	return r.workspace
}

// Register adds or replaces a tool by Info().Name.
func (r *Registry) Register(t tool.BaseTool) error {
	if t == nil {
		return fmt.Errorf("tools: nil tool")
	}
	info, err := t.Info(context.Background())
	if err != nil {
		return err
	}
	if info == nil || info.Name == "" {
		return fmt.Errorf("tools: tool missing name")
	}
	name := info.Name

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.byName[name]; !exists {
		r.order = append(r.order, name)
	}
	r.byName[name] = t
	return nil
}

// All returns tools in registration order.
func (r *Registry) All() []tool.BaseTool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]tool.BaseTool, 0, len(r.order))
	for _, n := range r.order {
		out = append(out, r.byName[n])
	}
	return out
}

// FilterByNames returns tools whose names appear in allow (preserving allow order).
// Duplicate names in allow are deduplicated by first occurrence.
func (r *Registry) FilterByNames(allow []string) ([]tool.BaseTool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	seen := make(map[string]bool)
	var names []string
	for _, n := range allow {
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		names = append(names, n)
	}

	var out []tool.BaseTool
	for _, n := range names {
		t, ok := r.byName[n]
		if !ok {
			return nil, fmt.Errorf("tools: unknown tool %q", n)
		}
		out = append(out, t)
	}
	return out, nil
}

// Names returns registered tool names in registration order (copy).
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]string(nil), r.order...)
}
