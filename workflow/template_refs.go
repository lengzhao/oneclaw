package workflow

import (
	"regexp"
	"strings"
)

var templateRefPattern = regexp.MustCompile(`\$(start|nodes|runtime)\.[A-Za-z0-9_.]+`)

// TemplateSourceChunks returns prompt, input, and every string reachable under params (values only), for template scanning.
func TemplateSourceChunks(n Node) []string {
	var out []string
	if s := strings.TrimSpace(n.Prompt); s != "" {
		out = append(out, s)
	}
	if s := strings.TrimSpace(n.Input); s != "" {
		out = append(out, s)
	}
	collectParamStringsForTemplates(n.Params, &out)
	return out
}

// TemplateReferencedNodes lists workflow node ids referenced as $nodes.<id>… in prompt, input, or params (recursive).
func TemplateReferencedNodes(n Node) []string {
	return templateNodeRefs(n)
}

func collectParamStringsForTemplates(v any, out *[]string) {
	switch x := v.(type) {
	case string:
		*out = append(*out, x)
	case []any:
		for _, el := range x {
			collectParamStringsForTemplates(el, out)
		}
	case map[string]any:
		for _, el := range x {
			collectParamStringsForTemplates(el, out)
		}
	}
}

func templateNodeRefs(n Node) []string {
	seen := map[string]struct{}{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" {
			seen[id] = struct{}{}
		}
	}
	for _, chunk := range TemplateSourceChunks(n) {
		for _, m := range templateRefPattern.FindAllString(chunk, -1) {
			add(nodeIDFromTemplateRef(m))
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out
}

func nodeIDFromTemplateRef(ref string) string {
	p := strings.Split(strings.TrimPrefix(ref, "$"), ".")
	if len(p) >= 2 && p[0] == "nodes" {
		return strings.TrimSpace(p[1])
	}
	return ""
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
