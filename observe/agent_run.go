package observe

import "context"

type agentRunAttrsKey struct{}

// AgentRunAttrs tags sub-agent runs for structured logs (FR-AGT-02, appendix §3.1).
type AgentRunAttrs struct {
	CorrelationID   string
	ParentSessionID string
	SubRunID        string
}

// WithAgentRunAttrs attaches parent/sub identifiers for ChatModelLogMiddleware.
func WithAgentRunAttrs(ctx context.Context, a AgentRunAttrs) context.Context {
	return context.WithValue(ctx, agentRunAttrsKey{}, a)
}

// AgentRunAttrsFromContext returns attrs attached by WithAgentRunAttrs.
func AgentRunAttrsFromContext(ctx context.Context) (AgentRunAttrs, bool) {
	a, ok := ctx.Value(agentRunAttrsKey{}).(AgentRunAttrs)
	return a, ok
}
