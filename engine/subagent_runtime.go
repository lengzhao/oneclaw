package engine

import (
	"os"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/toolhost"
)

// SubAgentRuntimeOpts configures ForkSubAgentRuntime for a nested workflow run (ExecuteSubAgentTurn).
type SubAgentRuntimeOpts struct {
	Turn              TurnContext
	DelegationDepth   int
	SubSessionRoot    string
	SessionSegment    string
	Agent             *catalog.Agent
	Bundle            *preturn.Bundle
	UserPrompt        string
	Catalog           *catalog.Catalog
	Cfg               *config.File
	UserDataRoot      string
	InstructionRoot   string
	WorkspacePath     string
	ToolRegistry      toolhost.Registry
	ChatAgent         *adk.ChatModelAgent
	ChatModel         model.ToolCallingChatModel
	AgentShellMeta    AgentShellMeta
	Stdout            *os.File
	RunStartedAt      time.Time
	UseMock           bool
	ProfileID         string
	ModelName         string
	CorrelationID     string
	OnAssistantChunk  func(content string)
	OnSubAgentChunk   func(correlationID, subRunID, agentType, chunk string)
}

// ForkSubAgentRuntime returns a RuntimeContext for a sub-agent workflow run.
// Shared Catalog/Cfg pointers are read-only; maps are fresh per nested run.
func ForkSubAgentRuntime(opts SubAgentRuntimeOpts) *RuntimeContext {
	return &RuntimeContext{
		TurnInputs: TurnInputs{
			Turn:                     opts.Turn,
			DelegationDepth:          opts.DelegationDepth,
			SessionRoot:              opts.SubSessionRoot,
			SessionSegment:           opts.SessionSegment,
			Agent:                    opts.Agent,
			Bundle:                   opts.Bundle,
			UserPrompt:               opts.UserPrompt,
			Catalog:                  opts.Catalog,
			Cfg:                      opts.Cfg,
			UserDataRoot:             opts.UserDataRoot,
			InstructionRoot:          opts.InstructionRoot,
			WorkspacePath:            opts.WorkspacePath,
			ToolRegistry:             opts.ToolRegistry,
			Stdout:                   opts.Stdout,
			RunStartedAt:             opts.RunStartedAt,
			UseMock:                  opts.UseMock,
			ProfileID:                opts.ProfileID,
			ModelName:                opts.ModelName,
			CorrelationID:            opts.CorrelationID,
			OnAssistantChunk:         opts.OnAssistantChunk,
			OnSubAgentAssistantChunk: opts.OnSubAgentChunk,
		},
		ADKRuntime: ADKRuntime{
			ChatAgent:      opts.ChatAgent,
			ChatModel:      opts.ChatModel,
			AgentShellMeta: opts.AgentShellMeta,
		},
		PromptScratch: PromptScratch{
			PromptTemplateData: make(map[string]any),
		},
		NodeScratch: NodeScratch{
			WorkflowNodeOutputs: make(map[string]map[string]any),
		},
	}
}
