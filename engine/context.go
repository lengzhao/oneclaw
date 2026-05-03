package engine

// TurnContext carries cross-cutting state for one user turn (architecture §3–4).
type TurnContext struct {
	AgentID string
	// ReplyMeta is a filtered snapshot of inbound metadata for nested tools/agents (e.g. schedule → Weixin token).
	ReplyMeta map[string]string
}
