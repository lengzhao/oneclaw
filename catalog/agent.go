package catalog

// Agent is a loaded Catalog entry (agents/*.md).
type Agent struct {
	AgentType string
	Name      string
	Description string

	Tools   []string
	Model   string
	MaxTurns int

	// Workspace is tools cwd mode for sub-agents: "shared" (default) or "private" (FR-AGT-06).
	Workspace string
	// InheritParentMemory injects parent MEMORY.md when true (default false; appendix §3.1).
	InheritParentMemory bool

	Body       string // markdown body (instruction prose)
	SourceStem string // filename stem for debugging
}
