package workflow

import (
	"fmt"
	"sort"
)

const reservedComposeNodePrefix = "_oneclaw_"

// ComposeIndegree counts predecessors for Eino compose: YAML incoming edges plus START when node is graph.entry.
func ComposeIndegree(g *Graph, nodeID string) int {
	if g == nil {
		return 0
	}
	n := 0
	if nodeID == g.Entry {
		n++
	}
	for _, e := range g.Edges {
		if e.To == nodeID {
			n++
		}
	}
	return n
}

// SinkNodes returns node ids with no outgoing YAML edges, sorted ascending.
func SinkNodes(g *Graph) []string {
	if g == nil {
		return nil
	}
	outgoing := map[string]bool{}
	for _, e := range g.Edges {
		outgoing[e.From] = true
	}
	var sinks []string
	for id := range g.Nodes {
		if !outgoing[id] {
			sinks = append(sinks, id)
		}
	}
	sort.Strings(sinks)
	return sinks
}

// OutgoingMap groups YAML edges by source node id.
func OutgoingMap(g *Graph) map[string][]string {
	m := map[string][]string{}
	if g == nil {
		return m
	}
	for _, e := range g.Edges {
		m[e.From] = append(m[e.From], e.To)
	}
	for k := range m {
		sort.Strings(m[k])
	}
	return m
}

// ValidateComposeFanOut rejects fan-out patterns Eino cannot express without per-edge keys:
// one source must not link to both a single-predecessor target and a multi-predecessor target.
func ValidateComposeFanOut(g *Graph) error {
	if g == nil {
		return nil
	}
	out := OutgoingMap(g)
	for from, tos := range out {
		var low, high bool
		for _, to := range tos {
			c := ComposeIndegree(g, to)
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
