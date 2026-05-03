// Package catalog loads agents/*.md definitions (FR-AGT-01/04).
package catalog

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed builtin/*.md
var builtinFS embed.FS

// Catalog maps agent_type -> definition (user overlays builtin).
type Catalog struct {
	Agents map[string]*Agent
}

// Get returns an agent by type or nil.
func (c *Catalog) Get(agentType string) *Agent {
	if c == nil || c.Agents == nil {
		return nil
	}
	return c.Agents[agentType]
}

// Load parses embedded defaults then user agents under agentDir (typically UserDataRoot/agents).
func Load(agentDir string) (*Catalog, error) {
	out := &Catalog{Agents: make(map[string]*Agent)}
	raw, rerr := builtinFS.ReadFile("builtin/default.md")
	if rerr == nil && len(raw) > 0 {
		a, perr := ParseAgentMarkdown("default", raw)
		if perr != nil {
			return nil, fmt.Errorf("builtin default: %w", perr)
		}
		out.Agents[a.AgentType] = a
	}
	if agentDir == "" {
		return out, nil
	}
	entries, err := os.ReadDir(agentDir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		if skipAgentMarkdown(name) {
			continue
		}
		path := filepath.Join(agentDir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		stem := strings.TrimSuffix(name, filepath.Ext(name))
		a, err := ParseAgentMarkdown(stem, raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		out.Agents[a.AgentType] = a
	}
	return out, nil
}

func skipAgentMarkdown(name string) bool {
	lower := strings.ToLower(name)
	if lower == "readme.md" {
		return true
	}
	return strings.HasSuffix(lower, ".readme.md")
}
