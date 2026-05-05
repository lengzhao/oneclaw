package adkhost

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	eino_tool "github.com/cloudwego/eino/components/tool"

	"github.com/lengzhao/oneclaw/tools"
)

// AgentOptions configures a single ChatModelAgent shell (phase 1).
type AgentOptions struct {
	Name          string
	Description   string
	Instruction   string
	MaxIterations int
	Handlers      []adk.ChatModelAgentMiddleware
}

// NewChatModelAgent wires ToolCallingChatModel + tools.Registry into adk.NewChatModelAgent.
func NewChatModelAgent(ctx context.Context, cm model.ToolCallingChatModel, reg *tools.Registry, opt AgentOptions) (*adk.ChatModelAgent, error) {
	if cm == nil {
		return nil, fmt.Errorf("adkhost: nil model")
	}
	if reg == nil {
		return nil, fmt.Errorf("adkhost: nil registry")
	}
	name := opt.Name
	if name == "" {
		name = "default"
	}
	max := opt.MaxIterations
	if max <= 0 {
		max = 20
	}
	tc := adk.ToolsConfig{}
	for _, bt := range reg.All() {
		if inv, ok := bt.(eino_tool.InvokableTool); ok {
			tc.Tools = append(tc.Tools, tools.WrapInvokableSoftErrors(inv))
			continue
		}
		tc.Tools = append(tc.Tools, bt)
	}
	cfg := &adk.ChatModelAgentConfig{
		Name:          name,
		Description:   opt.Description,
		Instruction:   opt.Instruction,
		Model:         cm,
		ToolsConfig:   tc,
		MaxIterations: max,
	}
	if len(opt.Handlers) > 0 {
		cfg.Handlers = opt.Handlers
	}
	return adk.NewChatModelAgent(ctx, cfg)
}
