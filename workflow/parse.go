package workflow

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseBytes parses workflow v2 YAML, expands steps sugar when nodes absent, merges defaults into node params.
func ParseBytes(raw []byte) (*Workflow, error) {
	var d rawDoc
	if err := yaml.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	w := &Workflow{
		SpecVersion: d.SpecVersion,
		ID:          strings.TrimSpace(d.ID),
		Description: d.Description,
		Defaults:    d.Defaults,
		Meta:        d.Meta,
		End:         strings.TrimSpace(d.End),
	}
	switch {
	case len(d.Nodes) > 0:
		w.Nodes = d.Nodes
	case len(d.Steps) > 0:
		nodes, err := expandSteps(d.Steps, false)
		if err != nil {
			return nil, err
		}
		w.Nodes = nodes
	default:
		return nil, fmt.Errorf("workflow: need nodes or steps")
	}
	mergeDefaultsIntoNodes(w.Defaults, w.Nodes)
	return w, nil
}

func expandSteps(steps []stepSugar, hostTurn bool) (map[string]Node, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("workflow: empty steps")
	}
	nodes := make(map[string]Node, len(steps))
	seen := map[string]bool{}
	var lastID string
	for i, s := range steps {
		if s.HostTurn != hostTurn {
			continue
		}
		if strings.TrimSpace(s.Use) == "" {
			return nil, fmt.Errorf("workflow: steps[%d] missing use", i)
		}
		id := strings.TrimSpace(s.ID)
		if id == "" {
			id = fmt.Sprintf("step_%d", i)
		}
		if seen[id] {
			return nil, fmt.Errorf("workflow: duplicate step id %q", id)
		}
		seen[id] = true
		n := Node{
			Use:       s.Use,
			AgentType: strings.TrimSpace(s.AgentType),
			Input:     s.Input,
			Prompt:    s.Prompt,
			Async:     s.Async,
			Params:    s.Params,
			DependsOn: append([]string(nil), s.DependsOn...),
		}
		if len(n.DependsOn) == 0 && lastID != "" {
			n.DependsOn = []string{lastID}
		}
		nodes[id] = n
		lastID = id
	}
	if len(nodes) == 0 {
		if hostTurn {
			return nil, nil
		}
		return nil, fmt.Errorf("workflow: no executable steps (all host_turn?)")
	}
	return nodes, nil
}

func mergeDefaultsIntoNodes(defaults map[string]any, nodes map[string]Node) {
	if len(defaults) == 0 || len(nodes) == 0 {
		return
	}
	for id, n := range nodes {
		if len(n.Params) == 0 {
			cp := shallowCloneMap(defaults)
			n.Params = cp
			nodes[id] = n
			continue
		}
		merged := shallowCloneMap(defaults)
		for k, v := range n.Params {
			merged[k] = v
		}
		n.Params = merged
		nodes[id] = n
	}
}

func shallowCloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
