package observe

import (
	"context"
	"log/slog"

	"github.com/cloudwego/eino/adk"
)

// ChatModelLogMiddleware logs model-call boundaries (FR-EINO-03 hook surface).
type ChatModelLogMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
}

// NewChatModelLogMiddleware returns a ChatModelAgentMiddleware with slog hooks.
func NewChatModelLogMiddleware() adk.ChatModelAgentMiddleware {
	return &ChatModelLogMiddleware{BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{}}
}

// BeforeModelRewriteState implements adk.ChatModelAgentMiddleware.
func (m *ChatModelLogMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	tools := 0
	if mc != nil {
		tools = len(mc.Tools)
	}
	slog.DebugContext(ctx, "adk.model_call", "phase", "before", "history_len", len(state.Messages), "tools", tools)
	return m.BaseChatModelAgentMiddleware.BeforeModelRewriteState(ctx, state, mc)
}

// AfterModelRewriteState implements adk.ChatModelAgentMiddleware.
func (m *ChatModelLogMiddleware) AfterModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	slog.DebugContext(ctx, "adk.model_call", "phase", "after", "history_len", len(state.Messages))
	return m.BaseChatModelAgentMiddleware.AfterModelRewriteState(ctx, state, mc)
}
