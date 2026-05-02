package session

import (
	"context"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/loop"
)

// beforeModelInjectMiddleware implements ADK BeforeModelRewriteState by calling
// [loop.Config.BeforeChatModel] so mid-turn user lines (e.g. TurnHub insert policy)
// are merged into ChatModelAgentState.Messages before each model request.
// ADK ReAct invokes GenModelInput only once at graph entry; later steps rely on this state.
type beforeModelInjectMiddleware struct {
	*adk.BaseChatModelAgentMiddleware
	cfg  *loop.Config
	step int
}

func maybeBeforeModelInjectHandler(cfg *loop.Config) adk.ChatModelAgentMiddleware {
	if cfg == nil || cfg.BeforeChatModel == nil {
		return nil
	}
	return &beforeModelInjectMiddleware{
		BaseChatModelAgentMiddleware: &adk.BaseChatModelAgentMiddleware{},
		cfg:                          cfg,
	}
}

func (m *beforeModelInjectMiddleware) BeforeModelRewriteState(ctx context.Context, state *adk.ChatModelAgentState, mc *adk.ModelContext) (context.Context, *adk.ChatModelAgentState, error) {
	_ = mc
	if m.cfg == nil || m.cfg.BeforeChatModel == nil || state == nil {
		return ctx, state, nil
	}
	m.step++
	msgs := state.Messages
	ptr := (*[]*schema.Message)(&msgs)
	beforeLen := len(*ptr)
	if err := m.cfg.BeforeChatModel(ctx, m.step, ptr); err != nil {
		return ctx, state, err
	}
	// Mirror hook appends (e.g. insert-policy user lines) into session memory so collapse/transcript
	// can retain them; slice shares the same message pointers as ADK state.
	var injected []*schema.Message
	if len(*ptr) > beforeLen {
		injected = append(injected, (*ptr)[beforeLen:]...)
	}
	loop.ApplyHistoryBudget(m.cfg.Budget, strings.TrimSpace(m.cfg.System), ptr)
	state.Messages = *ptr
	if m.cfg.Messages != nil && len(injected) > 0 {
		*m.cfg.Messages = append(*m.cfg.Messages, injected...)
	}
	return ctx, state, nil
}
