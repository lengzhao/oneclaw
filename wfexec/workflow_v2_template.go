package wfexec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/workflow"
)

var templateRefPattern = regexp.MustCompile(`\$(start|nodes|runtime)\.[A-Za-z0-9_.]+`)

const composeNodeTextInputField = "__node_text"
const composeNodeDataInputField = "__node_data"

func nodeInputForExec(state *compileState, graphInput map[string]any, node workflow.Node) (NodeInput, error) {
	var src string
	if strings.TrimSpace(node.Prompt) != "" {
		src = node.Prompt
	} else if strings.TrimSpace(node.Input) != "" {
		src = node.Input
	}
	text, err := renderNodeTemplate(state, graphInput, src)
	if err != nil {
		return NodeInput{}, err
	}
	return NodeInput{Text: text, Data: map[string]any{}}, nil
}

func renderNodeTemplate(state *compileState, graphInput map[string]any, src string) (string, error) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", nil
	}
	var ferr error
	out := templateRefPattern.ReplaceAllStringFunc(src, func(m string) string {
		v, err := lookupRef(state, graphInput, m)
		if err != nil {
			ferr = err
			return ""
		}
		return v
	})
	if ferr != nil {
		return "", ferr
	}
	return strings.TrimSpace(out), nil
}

func lookupRef(state *compileState, graphInput map[string]any, ref string) (string, error) {
	p := strings.Split(strings.TrimPrefix(ref, "$"), ".")
	if len(p) < 2 {
		return "", fmt.Errorf("wfexec: invalid template ref %q", ref)
	}
	switch p[0] {
	case "start":
		if p[1] == "user_prompt" {
			return strings.TrimSpace(state.rtx.EffectiveUserPrompt()), nil
		}
		return "", fmt.Errorf("wfexec: unsupported start ref %q", ref)
	case "nodes":
		if len(p) < 2 {
			return "", fmt.Errorf("wfexec: invalid node ref %q", ref)
		}
		nodeID := p[1]
		if len(p) == 2 || (len(p) == 3 && p[2] == "text") {
			if v := nodeTextFromGraphInput(graphInput, nodeID); v != "" {
				return v, nil
			}
			return "", nil
		}
		if len(p) >= 3 && p[2] != "text" {
			if v, ok := nodeDataFromGraphInput(graphInput, nodeID, p[2:]); ok {
				return strings.TrimSpace(fmt.Sprint(v)), nil
			}
			return "", nil
		}
		return "", nil
	case "runtime":
		switch strings.Join(p[1:], ".") {
		case "run_journal.current_turn_metadata":
			return strings.TrimSpace(state.rtx.CorrelationID), nil
		case "run_journal.path":
			sr := strings.TrimSpace(state.rtx.EffectiveSessionRoot())
			host := runtimeAgentType(state.rtx)
			corr := strings.TrimSpace(state.rtx.CorrelationID)
			if sr == "" || host == "" || corr == "" {
				return "", nil
			}
			return session.TurnRunJournalPath(sr, host, corr), nil
		case "user_data_root":
			return strings.TrimSpace(state.rtx.EffectiveUserDataRoot()), nil
		case "post_turn.ctx":
			s, err := BuildPostTurnCTXYAML(state.rtx)
			if err != nil {
				return "", err
			}
			return s, nil
		default:
			return "", fmt.Errorf("wfexec: unsupported runtime ref %q", ref)
		}
	default:
		return "", fmt.Errorf("wfexec: unknown template root %q", ref)
	}
}

func nodeTextFromGraphInput(graphInput map[string]any, nodeID string) string {
	if len(graphInput) == 0 || strings.TrimSpace(nodeID) == "" {
		return ""
	}
	raw, ok := graphInput[composeNodeTextInputField]
	if ok && raw != nil {
		switch m := raw.(type) {
		case map[string]any:
			if v, ok := m[nodeID]; ok {
				return strings.TrimSpace(fmt.Sprint(v))
			}
		case map[string]string:
			return strings.TrimSpace(m[nodeID])
		}
	}
	// Eino compose merges MapFields("Text", "__node_text.<id>") as a single top-level key.
	if v, ok := graphInput[composeNodeTextInputField+"."+nodeID]; ok {
		return strings.TrimSpace(fmt.Sprint(v))
	}
	return ""
}

func nodeDataFromGraphInput(graphInput map[string]any, nodeID string, fieldPath []string) (any, bool) {
	if len(graphInput) == 0 || strings.TrimSpace(nodeID) == "" || len(fieldPath) == 0 {
		return nil, false
	}
	raw, ok := graphInput[composeNodeDataInputField]
	if ok && raw != nil {
		if root, ok := raw.(map[string]any); ok {
			if nodeData, ok := root[nodeID]; ok {
				if v, ok := lookupAnyByPath(nodeData, fieldPath); ok {
					return v, true
				}
			}
		}
	}
	// Eino compose merges MapFields("Data", "__node_data.<id>") as one dotted top-level key.
	if flat, ok := graphInput[composeNodeDataInputField+"."+nodeID]; ok && flat != nil {
		return lookupAnyByPath(flat, fieldPath)
	}
	return nil, false
}

func lookupAnyByPath(root any, path []string) (any, bool) {
	cur := root
	for _, key := range path {
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, false
		}
		switch m := cur.(type) {
		case map[string]any:
			next, ok := m[key]
			if !ok {
				return nil, false
			}
			cur = next
		default:
			return nil, false
		}
	}
	return cur, true
}

func inferredTemplateDeps(node workflow.Node) []string {
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" {
			seen[s] = struct{}{}
		}
	}
	for _, dep := range node.DependsOn {
		add(dep)
	}
	for _, dep := range workflow.TemplateReferencedNodes(node) {
		add(dep)
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

func simpleTemplateNodeRefs(node workflow.Node) []string {
	seen := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s != "" {
			seen[s] = struct{}{}
		}
	}
	for _, chunk := range workflow.TemplateSourceChunks(node) {
		for _, m := range templateRefPattern.FindAllString(chunk, -1) {
			p := strings.Split(strings.TrimPrefix(m, "$"), ".")
			if len(p) >= 2 && p[0] == "nodes" && (len(p) == 2 || (len(p) == 3 && p[2] == "text")) {
				add(p[1])
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	return out
}

type advancedTemplateRef struct {
	NodeID    string
	FieldPath []string
}

func advancedTemplateNodeRefs(node workflow.Node) []advancedTemplateRef {
	seen := map[string]advancedTemplateRef{}
	for _, chunk := range workflow.TemplateSourceChunks(node) {
		for _, m := range templateRefPattern.FindAllString(chunk, -1) {
			p := strings.Split(strings.TrimPrefix(m, "$"), ".")
			if len(p) < 3 || p[0] != "nodes" {
				continue
			}
			if p[2] == "text" && len(p) == 3 {
				continue
			}
			nodeID := strings.TrimSpace(p[1])
			if nodeID == "" {
				continue
			}
			fieldPath := make([]string, 0, len(p)-2)
			valid := true
			for _, x := range p[2:] {
				x = strings.TrimSpace(x)
				if x == "" {
					valid = false
					break
				}
				fieldPath = append(fieldPath, x)
			}
			if !valid || len(fieldPath) == 0 {
				continue
			}
			key := nodeID + "::" + strings.Join(fieldPath, ".")
			seen[key] = advancedTemplateRef{NodeID: nodeID, FieldPath: fieldPath}
		}
	}
	out := make([]advancedTemplateRef, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	return out
}
