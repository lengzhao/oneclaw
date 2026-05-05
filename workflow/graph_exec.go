package workflow

import (
	"fmt"
	"sort"
)

const reservedComposeNodePrefix = "_oneclaw_"

// ComposeIndegree counts direct dependencies for a node.
func ComposeIndegree(w *Workflow, nodeID string) int {
	if w == nil {
		return 0
	}
	n, ok := w.Nodes[nodeID]
	if !ok {
		return 0
	}
	return len(nodeDeps(n))
}

// SinkNodes returns node ids that no other node depends on.
func SinkNodes(w *Workflow) []string {
	if w == nil {
		return nil
	}
	outgoing := map[string]bool{}
	for _, n := range w.Nodes {
		for _, dep := range nodeDeps(n) {
			outgoing[dep] = true
		}
	}
	var sinks []string
	for id := range w.Nodes {
		if !outgoing[id] {
			sinks = append(sinks, id)
		}
	}
	sort.Strings(sinks)
	return sinks
}

// OutgoingMap groups dependency edges by source node id.
func OutgoingMap(w *Workflow) map[string][]string {
	m := map[string][]string{}
	if w == nil {
		return m
	}
	for id, n := range w.Nodes {
		for _, dep := range nodeDeps(n) {
			m[dep] = append(m[dep], id)
		}
	}
	for k := range m {
		sort.Strings(m[k])
	}
	return m
}

// ValidateComposeFanOut keeps prior guardrails to avoid mixed fan-out complexity.
func ValidateComposeFanOut(w *Workflow) error {
	if w == nil {
		return nil
	}
	out := OutgoingMap(w)
	for from, tos := range out {
		var low, high bool
		for _, to := range tos {
			c := ComposeIndegree(w, to)
			if c >= 2 {
				high = true
			}
			if c == 1 {
				low = true
			}
		}
		if low && high {
			return fmt.Errorf("workflow: node %q fans out to both merge and non-merge targets (unsupported)", from)
		}
	}
	return nil
}
