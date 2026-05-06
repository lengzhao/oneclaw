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

// AgentShellMeta holds ChatModelAgent fields needed to rebuild after preparing the system instruction.
type AgentShellMeta struct {
	Name          string
	Description   string
	MaxIterations int
	Handlers      []adk.ChatModelAgentMiddleware
}

// WorkflowExec holds mutexes and per-node execution scratch for the compose driver.
type WorkflowExec struct {
	// ExecMu serializes handler bodies (sync nodes and async goroutines contend fairly).
	ExecMu sync.Mutex

	asyncMu    sync.Mutex
	asyncSlots map[string]*asyncHandlerSlot // lazy: async handler completion

	CurrentNodeID    string
	CurrentParams    map[string]any
	CurrentAsync     bool
	UserTurnAppended bool
}

// TurnInputs is host-injected per-turn state (mostly stable during the workflow run).
type TurnInputs struct {
	GoCtx context.Context
	Turn  TurnContext

	SessionRoot    string
	SessionSegment string
	// InboundMediaPaths carries channel inbound attachments (clawbridge InboundMessage.MediaPaths).
	// wfexec/adk_main may inject them into prompt text and/or multimodal user parts.
	InboundMediaPaths []string
	Agent          *catalog.Agent
	Bundle         *preturn.Bundle
	UserPrompt     string

	Catalog      *catalog.Catalog
	Cfg          *config.File
	UserDataRoot string
	// InstructionRoot is the resolved instructions directory for this agent/run.
	InstructionRoot string
	WorkspacePath   string
	ToolRegistry    toolhost.Registry // parent runtime tools (subset source for sub-agents / run_agent)

	DelegationDepth int

	Stdout           *os.File
	OnAssistantChunk func(content string) // optional streaming hook
	// OnSubAgentAssistantChunk streams nested agent output (optional).
	OnSubAgentAssistantChunk func(correlationID, subRunID, agentType, chunk string)

	RunStartedAt time.Time
	UseMock      bool
	ProfileID    string
	ModelName    string

	CorrelationID string // ties one CLI/turn invocation to sub-agent logs (optional; wfexec may synthesize if empty)

	// WorkflowMeta is a shallow copy of workflow.Meta at wfexec.Execute time (e.g. transcript_mode).
	WorkflowMeta map[string]any

	// PostAssistantRespond runs after on_respond appends the assistant transcript (phase 5 outbound); optional.
	PostAssistantRespond func(ctx context.Context, assistant string) error
}

// ADKRuntime holds the main chat agent and last assistant output for the turn workflow.
type ADKRuntime struct {
	ChatAgent      *adk.ChatModelAgent
	ChatModel      model.ToolCallingChatModel
	AgentShellMeta AgentShellMeta

	Assistant string // last model message content (adk_main)

	SawOnRespond bool // transcript flush delegated to on_respond node
}

// PromptScratch holds mutable prompt assembly state (PreparePrompt phase nodes).
type PromptScratch struct {
	// PromptTemplateData holds workflow node outputs: SkillsIndex/Tasks merge into system prompt; MemoryRecall is attached as an optional user message in adk_main. Layout is embedded by default; optional agents/<agent_type>.prompt.tmpl overrides.
	PromptTemplateData map[string]any

	// TranscriptReplayTurns is set by adk_main context prep or explicit load_transcript from per-agent *_transcript.jsonl files (trimmed). When nil, adk_main sends only EffectiveUserPrompt as one user message.
	TranscriptReplayTurns []session.TranscriptTurn
}

// NodeScratch holds structured outputs keyed by workflow graph node id (params.context workflow_node refs; see EmitNodeOutput).
type NodeScratch struct {
	WorkflowNodeOutputs map[string]map[string]any
}

// RuntimeContext is mutable per-turn state shared by workflow nodes.
// It composes embedded sub-structs by concern; promoted fields keep existing rtx.Field access stable across packages.
type RuntimeContext struct {
	WorkflowExec
	TurnInputs
	ADKRuntime
	PromptScratch
	NodeScratch
}
