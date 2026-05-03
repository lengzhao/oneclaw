package workflow

import (
	"fmt"
	"strings"
)

const supportedSpecVersion = 1

// Phase3Uses are built-in node kinds wfexec implements today.
var Phase3Uses = map[string]struct{}{
	"on_receive":           {},
	"load_prompt_md":       {},
	"load_memory_snapshot": {},
	"filter_tools":         {},
	"adk_main":             {},
	"on_respond":           {},
	"noop":                 {},
}

// Validate checks workflows-spec §11 baseline for phase 3 (no use:if runtime yet).
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
	g := &w.Graph
	if strings.TrimSpace(g.Entry) == "" {
		return fmt.Errorf("workflow: graph.entry required")
	}
	if len(g.Nodes) == 0 {
		return fmt.Errorf("workflow: graph.nodes required")
	}
	if _, ok := g.Nodes[g.Entry]; !ok {
		return fmt.Errorf("workflow: graph.entry %q not found in nodes", g.Entry)
	}
	for _, e := range g.Edges {
		if _, ok := g.Nodes[e.From]; !ok {
			return fmt.Errorf("workflow: edge from unknown node %q", e.From)
		}
		if _, ok := g.Nodes[e.To]; !ok {
			return fmt.Errorf("workflow: edge to unknown node %q", e.To)
		}
	}
	for id, n := range g.Nodes {
		if strings.TrimSpace(n.Use) == "" {
			return fmt.Errorf("workflow: node %q missing use", id)
		}
		if n.Use == "if" {
			return fmt.Errorf("workflow: node %q uses if branches (not implemented yet)", id)
		}
		if _, ok := Phase3Uses[n.Use]; !ok {
			return fmt.Errorf("workflow: unknown use %q on node %q (phase 3 whitelist)", n.Use, id)
		}
	}
	if err := validateDAG(g); err != nil {
		return err
	}
	if err := validateReachable(g); err != nil {
		return err
	}
	return nil
}

func validateDAG(g *Graph) error {
	indeg := map[string]int{}
	for id := range g.Nodes {
		indeg[id] = 0
	}
	for _, e := range g.Edges {
		indeg[e.To]++
	}
	if indeg[g.Entry] != 0 {
		return fmt.Errorf("workflow: entry node %q must have indegree 0", g.Entry)
	}
	// Kahn from entry-only reachable set would be better; use full node set and require single indegree-0.
	zeros := 0
	for id, d := range indeg {
		if d == 0 {
			zeros++
			if id != g.Entry {
				return fmt.Errorf("workflow: multiple sources — node %q has indegree 0 but is not entry", id)
			}
		}
	}
	if zeros != 1 {
		return fmt.Errorf("workflow: expected exactly one indegree-0 node (entry)")
	}
	order, err := TopoSort(g)
	if err != nil {
		return err
	}
	if len(order) != len(g.Nodes) {
		return fmt.Errorf("workflow: internal topo length mismatch")
	}
	return nil
}

func validateReachable(g *Graph) error {
	reach := map[string]bool{}
	var dfs func(string)
	dfs = func(id string) {
		if reach[id] {
			return
		}
		reach[id] = true
		for _, e := range g.Edges {
			if e.From == id {
				dfs(e.To)
			}
		}
	}
	dfs(g.Entry)
	for id := range g.Nodes {
		if !reach[id] {
			return fmt.Errorf("workflow: node %q not reachable from entry", id)
		}
	}
	return nil
}
