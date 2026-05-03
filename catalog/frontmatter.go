package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SplitYAMLFrontmatter parses leading ---\n...\n--- markdown-style frontmatter.
func SplitYAMLFrontmatter(raw []byte) (frontYAML []byte, body string, err error) {
	s := strings.TrimPrefix(string(raw), "\ufeff")
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "---") {
		return nil, s, nil
	}
	rest := strings.TrimPrefix(s[3:], "\n")
	if rest == "" {
		return nil, "", errors.New("catalog: empty frontmatter")
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, "", fmt.Errorf("catalog: unterminated frontmatter")
	}
	fm := rest[:end]
	body = strings.TrimSpace(rest[end+4:])
	return []byte(fm), body, nil
}

// AgentFrontmatter is the YAML block in agents/*.md.
type AgentFrontmatter struct {
	Name                      string   `yaml:"name,omitempty"`
	Description               string   `yaml:"description,omitempty"`
	Tools                     []string `yaml:"tools,omitempty"`
	Model                     string   `yaml:"model,omitempty"`
	MaxTurns                  int      `yaml:"max_turns,omitempty"`
}

// ParseAgentMarkdown extracts frontmatter + body. Catalog identity is always stem (filename without extension).
func ParseAgentMarkdown(stem string, raw []byte) (*Agent, error) {
	fmBytes, body, err := SplitYAMLFrontmatter(raw)
	if err != nil {
		return nil, err
	}
	a := &Agent{
		SourceStem: stem,
		Body:       body,
	}
	if len(bytes.TrimSpace(fmBytes)) == 0 {
		a.Body = strings.TrimSpace(string(raw))
		a.AgentType = stem
		return a, nil
	}
	var fm AgentFrontmatter
	if err := yaml.Unmarshal(fmBytes, &fm); err != nil {
		return nil, err
	}
	a.AgentType = stem
	a.Name = fm.Name
	a.Description = fm.Description
	a.Tools = fm.Tools
	a.Model = fm.Model
	a.MaxTurns = fm.MaxTurns
	if a.Name == "" {
		a.Name = a.AgentType
	}
	return a, nil
}
