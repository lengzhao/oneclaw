package workflow

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ParseBytes parses YAML, expands steps sugar when graph absent, merges defaults into nodes.
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
	}
	switch {
	case d.Graph != nil && (len(d.Graph.Nodes) > 0 || d.Graph.Entry != ""):
		w.Graph = *d.Graph
	case len(d.Steps) > 0:
		g, err := expandSteps(d.Steps)
		if err != nil {
			return nil, err
		}
		w.Graph = *g
	default:
		return nil, fmt.Errorf("workflow: need graph or steps")
	}
	mergeDefaultsIntoNodes(w.Defaults, &w.Graph)
	return w, nil
}

func expandSteps(steps []stepSugar) (*Graph, error) {
	if len(steps) == 0 {
		return nil, fmt.Errorf("workflow: empty steps")
	}
	g := &Graph{
		Nodes: make(map[string]Node),
		Edges: nil,
	}
	seen := map[string]bool{}
	var lastID string
	for i, s := range steps {
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
		g.Nodes[id] = Node{Use: s.Use, Params: s.Params, Async: s.Async}
		if lastID != "" {
			g.Edges = append(g.Edges, Edge{From: lastID, To: id})
		} else {
			g.Entry = id
		}
		lastID = id
	}
	if g.Entry == "" {
		g.Entry = lastID
	}
	return g, nil
}

func mergeDefaultsIntoNodes(defaults map[string]any, g *Graph) {
	if len(defaults) == 0 || g == nil {
		return
	}
	for id, n := range g.Nodes {
		if len(n.Params) == 0 {
			cp := shallowCloneMap(defaults)
			n.Params = cp
			g.Nodes[id] = n
			continue
		}
		merged := shallowCloneMap(defaults)
		for k, v := range n.Params {
			merged[k] = v
		}
		n.Params = merged
		g.Nodes[id] = n
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
