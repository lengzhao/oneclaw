package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// MetaHostTurnNodesYAML is the legacy meta key under skill_generator meta (string YAML fragment).
// Prefer top-level host_turn_nodes on that file.
const MetaHostTurnNodesYAML = "host_turn_nodes_yaml"

// MergeHostTurnNodesFromSkillGenerator merges host-only nodes from workflows/skill_generator.{yaml,yml}
// into the host workflow when host.ID is default.turn. Lookup order:
//  1. steps with host_turn: true (expanded with the same steps sugar)
//  2. Else top-level host_turn_nodes (legacy map of node id → Node)
//  3. Else meta.host_turn_nodes_yaml (legacy multiline string)
// Missing file or empty patch is a no-op.
func MergeHostTurnNodesFromSkillGenerator(catalogRoot string, host *Workflow) error {
	if host == nil || strings.TrimSpace(host.ID) != "default.turn" {
		return nil
	}
	patch, err := readSkillGeneratorHostTurnPatch(catalogRoot)
	if err != nil {
		return err
	}
	if len(patch) == 0 {
		return nil
	}
	if host.Nodes == nil {
		host.Nodes = map[string]Node{}
	}
	for id, n := range patch {
		host.Nodes[id] = n
	}
	return nil
}

func readSkillGeneratorHostTurnPatch(catalogRoot string) (map[string]Node, error) {
	root := filepath.Clean(strings.TrimSpace(catalogRoot))
	if root == "" {
		return nil, nil
	}
	var lastErr error
	for _, ext := range []string{".yaml", ".yml"} {
		p := filepath.Join(root, "workflows", "skill_generator"+ext)
		raw, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			lastErr = err
			continue
		}
		var doc struct {
			Steps         []stepSugar     `yaml:"steps"`
			HostTurnNodes map[string]Node `yaml:"host_turn_nodes"`
			Meta          map[string]any  `yaml:"meta"`
		}
		if err := yaml.Unmarshal(raw, &doc); err != nil {
			return nil, fmt.Errorf("read skill_generator workflow: %w", err)
		}
		if len(doc.Steps) > 0 {
			hostNodes, err := expandSteps(doc.Steps, true)
			if err != nil {
				return nil, err
			}
			if len(hostNodes) > 0 {
				return hostNodes, nil
			}
		}
		if len(doc.HostTurnNodes) > 0 {
			return doc.HostTurnNodes, nil
		}
		if len(doc.Meta) == 0 {
			return nil, nil
		}
		rawStr, ok := doc.Meta[MetaHostTurnNodesYAML]
		if !ok {
			return nil, nil
		}
		s, ok := rawStr.(string)
		if !ok {
			return nil, fmt.Errorf("workflow: skill_generator meta.%s must be a string when steps.host_turn/host_turn_nodes are absent", MetaHostTurnNodesYAML)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		var legacy map[string]Node
		if err := yaml.Unmarshal([]byte(s), &legacy); err != nil {
			return nil, fmt.Errorf("workflow: skill_generator meta.%s: %w", MetaHostTurnNodesYAML, err)
		}
		return legacy, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}
