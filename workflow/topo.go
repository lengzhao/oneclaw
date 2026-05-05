package workflow

import (
	"fmt"
	"sort"
)

// TopoSort returns nodes in Kahn order (stable tie-break by node id ascending).
func TopoSort(w *Workflow) ([]string, error) {
	indeg := map[string]int{}
	succ := map[string][]string{}
	for id := range w.Nodes {
		indeg[id] = 0
	}
	for id, n := range w.Nodes {
		for _, dep := range nodeDeps(n) {
			indeg[id]++
			succ[dep] = append(succ[dep], id)
		}
	}
	for _, outs := range succ {
		sort.Strings(outs)
	}

	var q []string
	for id, d := range indeg {
		if d == 0 {
			q = append(q, id)
		}
	}
	sort.Strings(q)
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
	if len(out) != len(w.Nodes) {
		return nil, fmt.Errorf("workflow: graph has a cycle or disconnected subgraph")
	}
	return out, nil
}
