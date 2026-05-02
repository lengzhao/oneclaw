package loop

import (
	"context"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

// Config drives one user turn for the session TurnRunner (Eino ADK): tools, transcript shape,
// budgets, and hooks share this struct with persistence helpers in this package.
type Config struct {
	Client *openai.Client
	Model  string
	System string
	// MaxTokens caps completion length per provider request (ADK / Chat Completions).
	MaxTokens int64
	// MaxSteps is the legacy alias for TurnMaxSteps where still wired (nested agents).
	MaxSteps int
	Messages *[]openai.ChatCompletionMessageParamUnion
	Registry *tools.Registry
	// ToolContext is bound for builtin tools (cwd, session host, nested depth).
	ToolContext *toolctx.Context
	CanUseTool  tools.CanUseTool
	// OutboundText publishes assistant-visible text per model step; nil skips (unused by ADK-only paths unless wired).
	OutboundText func(ctx context.Context, text string) error
	MemoryAgentMd           string
	InboundMeta             string
	InboundAttachmentChunks []InboundUserChunk
	UserLine                string
	Budget                  budget.Global
	ChatTransport           string
	ChatCompletionExtraJSON []byte
	EinoOpenAIAPIKey        string
	EinoOpenAIBaseURL       string
	ToolTrace               *ToolTraceSink
	OnToolLogged            func(ToolTraceEntry)
	SlimTranscript          func(assistantText string)
	SessionID               string
	Lifecycle               *LifecycleCallbacks
	BeforeModelStep         func(ctx context.Context, step int, msgs *[]openai.ChatCompletionMessageParamUnion) error
	TurnMaxSteps            int
}
