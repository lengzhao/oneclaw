package subagent

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Definition is a role template for run_agent (directory-driven, TS: AgentDefinition subset).
type Definition struct {
	AgentType   string   `yaml:"agent_type"`
	Description string   `yaml:"description"`
	Tools       []string `yaml:"tools"`
	MaxTurns    int      `yaml:"max_turns"`
	// Model, when non-empty after trim, overrides the host default for this agent's nested loop.
	Model string `yaml:"model"`
	// OmitMemoryInjection: when true, run_agent does not prepend memory user blocks (explore-style).
	OmitMemoryInjection bool `yaml:"omit_memory_injection"`
	SystemPrompt        string
	SourcePath          string
}

type frontmatterFields struct {
	AgentType           string   `yaml:"agent_type"`
	Name                string   `yaml:"name"`
	Description         string   `yaml:"description"`
	Tools               []string `yaml:"tools"`
	MaxTurns            int      `yaml:"max_turns"`
	Model               string   `yaml:"model"`
	OmitMemoryInjection bool     `yaml:"omit_memory_injection"`
}

// ParseAgentFile parses one markdown file: optional YAML frontmatter + body as system prompt.
func ParseAgentFile(path string, raw []byte) (Definition, error) {
	text := string(raw)
	text = strings.TrimPrefix(text, "\ufeff")
	body := strings.TrimSpace(text)

	var fm frontmatterFields
	if strings.HasPrefix(body, "---\n") || strings.HasPrefix(body, "---\r\n") {
		rest := body
		if strings.HasPrefix(rest, "---\r\n") {
			rest = rest[5:]
		} else {
			rest = rest[4:]
		}
		end := strings.Index(rest, "\n---")
		if end < 0 {
			return Definition{}, fmt.Errorf("subagent: unclosed frontmatter in %s", path)
		}
		yamlPart := rest[:end]
		body = strings.TrimSpace(rest[end+4:])
		if err := yaml.Unmarshal([]byte(yamlPart), &fm); err != nil {
			return Definition{}, fmt.Errorf("subagent: yaml frontmatter %s: %w", path, err)
		}
	}

	agentType := fm.AgentType
	if agentType == "" {
		agentType = fm.Name
	}
	if agentType == "" {
		base := filepath.Base(path)
		agentType = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return Definition{
		AgentType:           agentType,
		Description:         fm.Description,
		Tools:               fm.Tools,
		MaxTurns:            fm.MaxTurns,
		Model:               strings.TrimSpace(fm.Model),
		OmitMemoryInjection: fm.OmitMemoryInjection,
		SystemPrompt:        body,
		SourcePath:          path,
	}, nil
}
