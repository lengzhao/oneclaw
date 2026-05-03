package config

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// MergeYAMLMissingFile reads path as YAML (or starts empty), injects keys present in defaultsYAML
// only where missing (deep, non-destructive). Uses yaml.Node so line comments and key order on
// existing nodes are preserved when re-encoded.
func MergeYAMLMissingFile(path string, defaultsYAML []byte) error {
	var srcRoot yaml.Node
	if err := yaml.Unmarshal(defaultsYAML, &srcRoot); err != nil {
		return err
	}
	srcMap := docMapping(&srcRoot)
	if srcMap == nil {
		return fmt.Errorf("config: defaults must be a mapping at the document root")
	}

	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var dstRoot yaml.Node
	if err == nil && len(bytes.TrimSpace(raw)) > 0 {
		if err := yaml.Unmarshal(raw, &dstRoot); err != nil {
			return err
		}
	} else {
		dstRoot.Kind = yaml.DocumentNode
		dstRoot.Content = []*yaml.Node{
			{Kind: yaml.MappingNode, Tag: "!!map"},
		}
	}

	dstMap := docMapping(&dstRoot)
	if dstMap == nil {
		return fmt.Errorf("config: %q must be a mapping at the document root", path)
	}

	mergeMissingMappings(dstMap, srcMap)

	out, err := yaml.Marshal(&dstRoot)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}

func docMapping(root *yaml.Node) *yaml.Node {
	if root == nil || root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil
	}
	n := root.Content[0]
	if n.Kind != yaml.MappingNode {
		return nil
	}
	return n
}

func mergeMissingMappings(dst, src *yaml.Node) {
	if dst == nil || src == nil || dst.Kind != yaml.MappingNode || src.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(src.Content); i += 2 {
		sk := src.Content[i]
		sv := src.Content[i+1]
		if sk.Kind != yaml.ScalarNode {
			continue
		}
		key := sk.Value
		valIdx, ok := mappingValueIndex(dst, key)
		if !ok {
			dst.Content = append(dst.Content, cloneYAMLNode(sk), cloneYAMLNode(sv))
			continue
		}
		dv := dst.Content[valIdx]
		if dv.Kind == yaml.MappingNode && sv.Kind == yaml.MappingNode {
			mergeMissingMappings(dv, sv)
		}
	}
}

// mappingValueIndex returns the Content index of the value node for scalar key, or false.
func mappingValueIndex(m *yaml.Node, key string) (int, bool) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		kn := m.Content[i]
		if kn.Kind == yaml.ScalarNode && kn.Value == key {
			return i + 1, true
		}
	}
	return 0, false
}

func cloneYAMLNode(n *yaml.Node) *yaml.Node {
	if n == nil {
		return nil
	}
	out := &yaml.Node{
		Kind:        n.Kind,
		Style:       n.Style,
		Tag:         n.Tag,
		Value:       n.Value,
		Anchor:      n.Anchor,
		HeadComment: n.HeadComment,
		LineComment: n.LineComment,
		FootComment: n.FootComment,
	}
	if len(n.Content) > 0 {
		out.Content = make([]*yaml.Node, len(n.Content))
		for i, c := range n.Content {
			out.Content[i] = cloneYAMLNode(c)
		}
	}
	return out
}
