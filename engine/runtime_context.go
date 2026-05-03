package engine

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/toolhost"
)

// AgentShellMeta holds ChatModelAgent fields needed to rebuild after mutating system instruction (e.g. load_memory_snapshot).
type AgentShellMeta struct {
	Name          string
	Description   string
	MaxIterations int
	Handlers      []adk.ChatModelAgentMiddleware
}

// RuntimeContext is mutable per-turn state shared by workflow nodes.
type RuntimeContext struct {
	// ExecMu serializes handler bodies (sync nodes and async goroutines contend fairly).
	ExecMu sync.Mutex

	asyncMu    sync.Mutex
	asyncSlots map[string]*asyncHandlerSlot // lazy: async handler completion

	GoCtx context.Context
	Turn  TurnContext

	SessionRoot    string
	SessionSegment string
	Agent          *catalog.Agent
	Bundle         *preturn.Bundle
	UserPrompt     string

	// Catalog / config / roots for workflow nodes that spawn other agents (use: agent).
	Catalog         *catalog.Catalog
	Cfg             *config.File
	UserDataRoot    string
	InstructionRoot string
	WorkspacePath   string
	ToolRegistry    toolhost.Registry // parent runtime tools (subset source for sub-agents / run_agent)
	CurrentNodeID   string
	CurrentParams   map[string]any
	CurrentAsync    bool

	ChatAgent *adk.ChatModelAgent
	// ChatModel backs rebuilding ChatAgent after instruction mutation (workflow load_memory_snapshot).
	ChatModel      model.ToolCallingChatModel
	AgentShellMeta AgentShellMeta

	Assistant string // last model message content (adk_main)

	Stdout           *os.File
	OnAssistantChunk func(content string) // optional streaming hook
	// OnSubAgentAssistantChunk streams nested agent output (optional).
	OnSubAgentAssistantChunk func(correlationID, subRunID, agentType, chunk string)

	RunStartedAt time.Time
	UseMock      bool
	ProfileID    string
	ModelName    string

	// CorrelationID ties one CLI/turn invocation to sub-agent logs (optional; wfexec may synthesize if empty).
	CorrelationID string

	SawOnRespond bool // transcript flush delegated to on_respond node

	// PostAssistantRespond runs after on_respond appends the assistant transcript (phase 5 outbound); optional.
	PostAssistantRespond func(ctx context.Context, assistant string) error

	// PromptTemplateData holds workflow node outputs: SkillsIndex/Tasks merge into system prompt; MemoryRecall is attached as an optional user message in adk_main. Layout is embedded by default; optional agents/<agent_type>.prompt.tmpl overrides.
	PromptTemplateData map[string]any

	// TranscriptReplayTurns is set by workflow load_transcript from transcript.jsonl (trimmed). When nil, adk_main sends only EffectiveUserPrompt as one user message.
	TranscriptReplayTurns []session.TranscriptTurn
}
