package subagent

import (
	"fmt"

	"github.com/lengzhao/oneclaw/tools"
)

// FilterRegistry keeps only named tools. Empty allow means all tools from parent.
func FilterRegistry(parent *tools.Registry, allow []string) (*tools.Registry, error) {
	if parent == nil {
		return nil, fmt.Errorf("subagent: nil parent registry")
	}
	if len(allow) == 0 {
		return cloneRegistry(parent)
	}
	out := tools.NewRegistry()
	for _, name := range allow {
		t, ok := parent.Get(name)
		if !ok {
			return nil, fmt.Errorf("subagent: tool %q not in parent registry", name)
		}
		if err := out.Register(t); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func cloneRegistry(parent *tools.Registry) (*tools.Registry, error) {
	out := tools.NewRegistry()
	for _, name := range parent.ToolNames() {
		t, ok := parent.Get(name)
		if !ok {
			continue
		}
		if err := out.Register(t); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// WithoutMetaTools removes nested-agent tools so depth is enforced by availability.
func WithoutMetaTools(r *tools.Registry, names ...string) (*tools.Registry, error) {
	if r == nil {
		return nil, fmt.Errorf("subagent: nil registry")
	}
	skip := make(map[string]struct{}, len(names))
	for _, n := range names {
		skip[n] = struct{}{}
	}
	out := tools.NewRegistry()
	for _, name := range r.ToolNames() {
		if _, drop := skip[name]; drop {
			continue
		}
		t, ok := r.Get(name)
		if !ok {
			continue
		}
		if err := out.Register(t); err != nil {
			return nil, err
		}
	}
	return out, nil
}
