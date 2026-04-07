package config

// MCPFile configures MCP client (external MCP servers whose tools are exposed to the model).
// See docs in config/project_init.example.yaml.
type MCPFile struct {
	// Enabled, when set in a merged layer, turns MCP on. Nil after merge means disabled.
	Enabled *bool `yaml:"enabled"`
	// MaxInlineTextRunes caps MCP text returned inline; larger bodies are written under .oneclaw/artifacts/mcp/.
	MaxInlineTextRunes int `yaml:"max_inline_text_runes"`
	Servers            map[string]MCPServerFile `yaml:"servers"`
}

// MCPServerFile is one MCP server (stdio, sse, or http transport).
type MCPServerFile struct {
	Enabled  bool              `yaml:"enabled"`
	Command  string            `yaml:"command"`
	Args     []string          `yaml:"args"`
	Env      map[string]string `yaml:"env"`
	EnvFile  string            `yaml:"env_file"`
	Type     string            `yaml:"type"` // stdio | sse | http; auto if empty
	URL      string            `yaml:"url"`
	Headers  map[string]string `yaml:"headers"`
}

func mergeMCP(dst *MCPFile, src MCPFile) {
	if src.Enabled != nil {
		dst.Enabled = src.Enabled
	}
	if src.MaxInlineTextRunes != 0 {
		dst.MaxInlineTextRunes = src.MaxInlineTextRunes
	}
	if len(src.Servers) > 0 {
		if dst.Servers == nil {
			dst.Servers = make(map[string]MCPServerFile)
		}
		for k, v := range src.Servers {
			dst.Servers[k] = v
		}
	}
}

// MCPEnabled reports whether merged config enables MCP (explicit true in some layer).
func (r *Resolved) MCPEnabled() bool {
	if r == nil {
		return false
	}
	return r.merged.MCP.Enabled != nil && *r.merged.MCP.Enabled
}

// MCP returns the merged MCP block (servers map may be nil).
func (r *Resolved) MCP() MCPFile {
	if r == nil {
		return MCPFile{}
	}
	return r.merged.MCP
}
