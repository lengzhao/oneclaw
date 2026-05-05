package catalog

import "strings"

// Agent is a loaded Catalog entry (agents/*.md).
type Agent struct {
	AgentType   string
	Name        string
	Description string

	Tools    []string
	Model    string
	MaxTurns int

	// ReferencedSkillIDs lists skill folder names under UserDataRoot/skills/<id>/ whose SKILL.md is injected into PreTurn (YAML key `skills`).
	ReferencedSkillIDs []string

	// Workspace is tools cwd mode for sub-agents: "shared" (default) or "private" (FR-AGT-06).
	Workspace string
	// InheritParentMemory injects parent MEMORY.md when true (default false; appendix §3.1).
	InheritParentMemory bool
	// ContextProfile customizes the default full context by disabling named blocks.
	ContextProfile ContextProfile

	Body       string // markdown body (instruction prose)
	SourceStem string // filename stem for debugging
}

// ContextProfile starts from the built-in full agent context and disables named blocks.
// Empty profile means "full context".
type ContextProfile struct {
	Disable []string `yaml:"disable,omitempty"`
}

// Disabled reports whether a context block is disabled by profile.
func (p ContextProfile) Disabled(block string) bool {
	block = normalizeContextBlock(block)
	if block == "" {
		return false
	}
	for _, s := range p.Disable {
		if normalizeContextBlock(s) == block {
			return true
		}
	}
	return false
}

func normalizeContextBlock(s string) string {
	switch s {
	case "":
		return ""
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}
