package subagent

import (
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Catalog maps agent_type -> definition (user files override builtins with same name).
type Catalog struct {
	byName map[string]Definition
}

// LoadCatalog loads `.oneclaw/agents/*.md` under cwd and merges built-ins.
func LoadCatalog(cwd string) *Catalog {
	byName := make(map[string]Definition)
	for _, d := range builtinDefinitions() {
		byName[d.AgentType] = d
	}
	dir := filepath.Join(cwd, ".oneclaw", "agents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			slog.Warn("subagent.catalog.read_dir", "dir", dir, "err", err)
		}
		return &Catalog{byName: byName}
	}
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			slog.Warn("subagent.catalog.read_file", "path", path, "err", err)
			continue
		}
		def, err := ParseAgentFile(path, raw)
		if err != nil {
			slog.Warn("subagent.catalog.parse", "path", path, "err", err)
			continue
		}
		if def.SystemPrompt == "" {
			slog.Warn("subagent.catalog.empty_body", "agent_type", def.AgentType, "path", path)
			continue
		}
		byName[def.AgentType] = def
	}
	return &Catalog{byName: byName}
}

// Get returns a definition by agent_type.
func (c *Catalog) Get(name string) (Definition, bool) {
	if c == nil {
		return Definition{}, false
	}
	d, ok := c.byName[name]
	return d, ok
}

// ListNames returns sorted agent_type names for tool descriptions.
func (c *Catalog) ListNames() []string {
	if c == nil {
		return nil
	}
	out := make([]string, 0, len(c.byName))
	for n := range c.byName {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func builtinDefinitions() []Definition {
	return []Definition{
		{
			AgentType:           "general-purpose",
			Description:         "Default delegated worker with the same tool surface as the parent (minus nested agent tools when at max depth).",
			Tools:               nil,
			MaxTurns:            0,
			OmitMemoryInjection: false,
			SystemPrompt: `You are a sub-agent delegated by the main Oneclaw session. Complete the assigned task using tools.
Use absolute paths when helpful. Be concise in the final answer; the main agent only sees your last reply text.
Do not mention internal tool names unless relevant.`,
			SourcePath: "",
		},
		{
			AgentType:           "explore",
			Description:         "Read-only exploration: read_file and grep only.",
			Tools:               []string{"read_file", "grep"},
			MaxTurns:            0,
			OmitMemoryInjection: true,
			SystemPrompt: `You are the Explore sub-agent. Inspect the codebase read-only: use read_file and grep only.
Return a structured, concise summary for the parent agent. Do not modify files or run shell.`,
			SourcePath: "",
		},
	}
}
