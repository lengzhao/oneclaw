package adkhost

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/components/model"

	"github.com/lengzhao/oneclaw/config"
)

// NewToolCallingChatModel returns a stub or OpenAI-compatible model from a resolved profile.
func NewToolCallingChatModel(ctx context.Context, prof *config.ModelProfile, useMock bool) (model.ToolCallingChatModel, error) {
	if prof == nil {
		return nil, fmt.Errorf("adkhost: nil model profile")
	}
	if useMock {
		return NewStubChatModel("Hello from oneclaw stub model."), nil
	}
	return NewOpenAIChatModel(ctx, prof)
}

// MaxAgentIterations returns a positive iteration cap from config defaults.
func MaxAgentIterations(cfg *config.File) int {
	if cfg == nil {
		return 100
	}
	n := cfg.Runtime.MaxAgentIterations
	if n <= 0 {
		return 100
	}
	return n
}

// MaxAgentIterationsOrCatalog uses catalogMaxTurns when > 0; otherwise [MaxAgentIterations].
func MaxAgentIterationsOrCatalog(cfg *config.File, catalogMaxTurns int) int {
	if catalogMaxTurns > 0 {
		return catalogMaxTurns
	}
	return MaxAgentIterations(cfg)
}

// MaxDelegationDepth returns the configured max nested run_agent depth (>= 1).
func MaxDelegationDepth(cfg *config.File) int {
	if cfg == nil {
		return 3
	}
	n := cfg.Runtime.MaxDelegationDepth
	if n <= 0 {
		return 3
	}
	return n
}
