package loop

import (
	"context"

	"github.com/openai/openai-go"
)

// ModelStepEndInfo is passed to LifecycleCallbacks.OnModelStepEnd after each model API attempt.
type ModelStepEndInfo struct {
	Model string // cfg.Model for this RunTurn
	OK    bool
	// AssistantVisible is user-visible assistant text for this step (content/refusal); empty if none or before response.
	AssistantVisible string
	// ToolCallsJSON is a JSON array of {id,name,arguments} when the model returned tool calls; empty otherwise.
	ToolCallsJSON string
	FinishReason     string
	ToolCallsCount   int
	DurationMs       int64
	PromptTokens     int64
	CompletionTokens int64
	TotalTokens      int64
	Err              error
	// BeforeRequestCancelled is true when ctx was already done before the model API call (no network request).
	BeforeRequestCancelled bool
}

// LifecycleCallbacks are optional hooks for observability (notify, tracing). All must be nil-safe for callers.
type LifecycleCallbacks struct {
	// OnModelStepStart is invoked immediately before each Chat Completions request.
	// requestMessages is the full payload (system + history) for this step.
	OnModelStepStart func(ctx context.Context, step, toolDefinitionsCount int, requestMessages []openai.ChatCompletionMessageParamUnion)
	OnModelStepEnd   func(ctx context.Context, step int, end ModelStepEndInfo)
	OnToolStart      func(ctx context.Context, modelStep int, toolUseID, toolName, argsPreview string)
}
