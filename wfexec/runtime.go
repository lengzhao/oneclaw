package wfexec

import (
	"context"
	"os"
	"time"

	"github.com/cloudwego/eino/adk"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/preturn"
)

// RuntimeContext is mutable per-turn state shared by workflow nodes.
type RuntimeContext struct {
	GoCtx          context.Context
	Turn           engine.TurnContext
	SessionRoot    string
	SessionSegment string
	Agent          *catalog.Agent
	Bundle         *preturn.Bundle
	UserPrompt     string

	ChatAgent *adk.ChatModelAgent

	Assistant string // last model message content (adk_main)

	Stdout       *os.File
	OnAssistantChunk func(content string) // optional streaming hook

	RunStartedAt time.Time
	UseMock      bool
	ProfileID    string
	ModelName    string

	SawOnRespond bool // transcript flush delegated to on_respond node
}
