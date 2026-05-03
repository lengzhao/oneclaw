package engine

import (
	"context"
	"os"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/toolhost"
)

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
}
