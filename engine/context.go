package engine

// TurnContext carries cross-cutting state for one user turn (architecture §3–4).
type TurnContext struct {
	AgentID             string
	EvolutionSuppressed bool
	Depth               int
}
