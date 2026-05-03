package workflow

import (
	"fmt"
	"sort"
)

// TopoSort returns nodes in Kahn order (stable tie-break by node id ascending).
func TopoSort(g *Graph) ([]string, error) {
	indeg := map[string]int{}
	succ := map[string][]string{}
	for id := range g.Nodes {
		indeg[id] = 0
	}
	for _, e := range g.Edges {
		indeg[e.To]++
		succ[e.From] = append(succ[e.From], e.To)
	}
	for _, outs := range succ {
		sort.Strings(outs)
	}

	q := []string{g.Entry}
	if indeg[g.Entry] != 0 {
		return nil, fmt.Errorf("workflow: cycle or bad entry indegree")
	}
	var out []string
	for len(q) > 0 {
		id := q[0]
		q = q[1:]
		out = append(out, id)
		for _, to := range succ[id] {
			indeg[to]--
			if indeg[to] == 0 {
				q = append(q, to)
				sort.Strings(q)
			}
		}
	}
	if len(out) != len(g.Nodes) {
		return nil, fmt.Errorf("workflow: graph has a cycle or disconnected subgraph")
	}
	return out, nil
}
