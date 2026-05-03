package catalog

// Agent is a loaded Catalog entry (agents/*.md).
type Agent struct {
	AgentType string
	Name      string
	Description string

	Tools   []string
	Model   string
	MaxTurns int

	Body       string // markdown body (instruction prose)
	SourceStem string // filename stem for debugging
}
