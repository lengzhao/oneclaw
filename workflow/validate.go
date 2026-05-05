package workflow

import (
	"fmt"
	"strings"
)

const supportedSpecVersion = 2

// Validate checks workflow v2 baseline.
func Validate(w *Workflow) error {
	if w == nil {
		return fmt.Errorf("workflow: nil document")
	}
	if w.SpecVersion != supportedSpecVersion {
		return fmt.Errorf("workflow: unsupported workflow_spec_version %d (want %d)", w.SpecVersion, supportedSpecVersion)
	}
	if strings.TrimSpace(w.ID) == "" {
		return fmt.Errorf("workflow: missing id")
	}
	if len(w.Nodes) == 0 {
		return fmt.Errorf("workflow: nodes required")
	}
	for id, n := range w.Nodes {
		if strings.HasPrefix(id, reservedComposeNodePrefix) {
			return fmt.Errorf("workflow: node id %q uses reserved prefix %q", id, reservedComposeNodePrefix)
		}
		if lid := strings.ToLower(strings.TrimSpace(id)); lid == "start" || lid == "end" {
			return fmt.Errorf("workflow: node id %q is reserved for compose START/END", id)
		}
		if strings.TrimSpace(n.Use) == "" {
			return fmt.Errorf("workflow: node %q missing use", id)
		}
		if _, ok := AllowedUses[n.Use]; !ok {
			return fmt.Errorf("workflow: unknown use %q on node %q (builtin whitelist)", n.Use, id)
		}
		if n.Use == "agent_task" {
			at := strings.TrimSpace(n.AgentType)
			if at == "" {
				at = AgentTypeParam(n.Params)
			}
			if at == "" {
				return fmt.Errorf("workflow: node %q (use: agent_task) requires agent_type", id)
			}
		}
		for _, dep := range n.DependsOn {
			dep = strings.TrimSpace(dep)
			if dep == "" {
				continue
			}
			if _, ok := w.Nodes[dep]; !ok {
				return fmt.Errorf("workflow: node %q depends_on unknown node %q", id, dep)
			}
		}
		for _, ref := range templateNodeRefs(n) {
			if _, ok := w.Nodes[ref]; !ok {
				return fmt.Errorf("workflow: node %q references unknown node %q", id, ref)
			}
		}
	}
	if strings.TrimSpace(w.End) != "" {
		if _, ok := w.Nodes[strings.TrimSpace(w.End)]; !ok {
			return fmt.Errorf("workflow: end node %q not found", w.End)
		}
	}
	if err := validateDAG(w); err != nil {
		return err
	}
	if err := ValidateComposeFanOut(w); err != nil {
		return err
	}
	return nil
}

func validateDAG(w *Workflow) error {
	order, err := TopoSort(w)
	if err != nil {
		return err
	}
	if len(order) != len(w.Nodes) {
		return fmt.Errorf("workflow: internal topo length mismatch")
	}
	return nil
}
