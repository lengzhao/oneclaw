package workflow

import (
	"regexp"
	"strings"
)

var templateRefPattern = regexp.MustCompile(`\$(start|nodes|runtime)\.[A-Za-z0-9_.]+`)

func templateNodeRefs(n Node) []string {
	var raw string
	if strings.TrimSpace(n.Prompt) != "" {
		raw = n.Prompt
	} else {
		raw = n.Input
	}
	seen := map[string]struct{}{}
	for _, m := range templateRefPattern.FindAllString(raw, -1) {
		p := strings.Split(strings.TrimPrefix(m, "$"), ".")
		if len(p) >= 2 && p[0] == "nodes" {
			id := strings.TrimSpace(p[1])
			if id != "" {
				seen[id] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

func nodeDeps(n Node) []string {
	seen := map[string]struct{}{}
	for _, dep := range n.DependsOn {
		dep = strings.TrimSpace(dep)
		if dep != "" {
			seen[dep] = struct{}{}
		}
	}
	for _, dep := range templateNodeRefs(n) {
		if dep != "" {
			seen[dep] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for dep := range seen {
		out = append(out, dep)
	}
	return out
}
