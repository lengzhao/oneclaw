package loop

import (
	"context"

	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
)

// Config drives one user turn for the session TurnRunner (Eino ADK): tools, transcript shape,
// budgets, and hooks share this struct with persistence helpers in this package.
type Config struct {
	Model  string
	System string
	// MaxTokens caps completion length per provider request (ADK / Chat Completions).
	MaxTokens int64
	// MaxSteps is the legacy alias for TurnMaxSteps where still wired (nested agents).
	MaxSteps int
	Messages *[]*schema.Message
	Registry *tools.Registry
	// ToolContext is bound for builtin tools (cwd, session host, nested depth).
	ToolContext *toolctx.Context
	CanUseTool  tools.CanUseTool
	// OutboundText publishes assistant-visible final reply text; nil skips.
	OutboundText            func(ctx context.Context, text string) error
	MemoryAgentMd           string
	InboundMeta             string
	InboundAttachmentChunks []InboundUserChunk
	UserLine                string
	Budget                  budget.Global
	EinoOpenAIAPIKey        string
	EinoOpenAIBaseURL       string
	SlimTranscript          func(assistantText string)
	SessionID               string
	// BeforeChatModel runs immediately before each underlying ChatModel request (Eino ADK:
	// ChatModelAgentMiddleware.BeforeModelRewriteState). Optional; e.g. TurnPolicyInsert
	// drains mid-turn user lines into msgs.
	BeforeChatModel func(ctx context.Context, step int, msgs *[]*schema.Message) error
	TurnMaxSteps    int
}
