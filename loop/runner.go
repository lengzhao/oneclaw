package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/model"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/usageledger"
	"github.com/openai/openai-go"
	"golang.org/x/sync/errgroup"
)

// Config drives one user turn (may include multiple model round-trips for tools).
type Config struct {
	Client    *openai.Client
	Model     string
	System    string
	MaxTokens int64
	// MaxSteps is the number of model calls per user turn. The last call is made without tools
	// so the model can only answer in text; earlier calls include tools when MaxSteps > 1.
	MaxSteps    int
	Messages    *[]openai.ChatCompletionMessageParamUnion
	Registry    *tools.Registry
	ToolContext *toolctx.Context
	CanUseTool  tools.CanUseTool
	// OutboundText publishes assistant-visible text per model step; nil skips.
	OutboundText func(ctx context.Context, text string) error
	// MemoryAgentMd / MemoryRecall are optional extra user messages before the real user turn (phase B).
	MemoryAgentMd string
	MemoryRecall  string
	// InboundMeta is optional channel routing context as a user-shaped block (see session orchestration).
	InboundMeta string
	// InboundAttachmentChunks are attachment-derived user messages (read_file hints and/or multimodal parts).
	InboundAttachmentChunks []InboundUserChunk
	// UserLine is the primary user message after orchestration (e.g. attachment-only placeholder). If empty, TrimSpace(in.Content) is used.
	UserLine string
	// Budget trims transcript before each model call when Enabled (from rtopts / config).
	Budget budget.Global
	// ChatTransport overrides default transport when non-empty (e.g. from config).
	ChatTransport string
	// ToolTrace, when non-nil, records slim per-tool rows for this RunTurn only (not sent to the model).
	ToolTrace *ToolTraceSink
	// OnToolLogged, when non-nil, called synchronously after each tool completes (e.g. append-only JSONL).
	OnToolLogged func(ToolTraceEntry)
	// SlimTranscript, if non-nil, called once when RunTurn finishes successfully with the final
	// assistant-visible reply (after any tool rounds). User turn is recorded before the model loop
	// by the caller (Claude Code–style: recordTranscript before query). Not called on error or abort.
	SlimTranscript func(assistantText string)
	// SessionID is optional; when set with CWD on ToolContext, usage is written under .oneclaw/usage/.
	SessionID string
	// Lifecycle optional hooks (model steps, tool execute start). Nil fields are skipped.
	Lifecycle *LifecycleCallbacks
}

func logOutboundEmit(op string, err error) {
	if err != nil {
		slog.Warn("loop.outbound.emit_failed", "op", op, "err", err)
	}
}

// RunTurn appends a user message from in.Content, then runs model ↔ tool until stop or limits.
func RunTurn(ctx context.Context, cfg Config, in bus.InboundMessage) (err error) {
	if cfg.Messages == nil {
		return fmt.Errorf("loop: Config.Messages is nil")
	}
	if cfg.ToolContext != nil {
		cfg.ToolContext.ApplyTurnInboundToToolContext(in)
	}

	msgs := cfg.Messages
	recordSlimTranscript := func() {
		if cfg.SlimTranscript == nil {
			return
		}
		cfg.SlimTranscript(LastAssistantDisplay(*msgs))
	}
	userLine := strings.TrimSpace(cfg.UserLine)
	if userLine == "" {
		userLine = strings.TrimSpace(in.Content)
	}
	AppendTurnUserMessages(msgs, cfg.MemoryAgentMd, cfg.MemoryRecall, cfg.InboundMeta, cfg.InboundAttachmentChunks, userLine)

	maxSteps := cfg.MaxSteps
	if maxSteps < 1 {
		maxSteps = 32
	}

	for step := 0; step < maxSteps; step++ {
		offerTools := maxSteps > 1 && step < maxSteps-1

		ApplyHistoryBudget(cfg.Budget, cfg.System, msgs)
		select {
		case <-ctx.Done():
			if cfg.Lifecycle != nil && (cfg.Lifecycle.OnModelStepStart != nil || cfg.Lifecycle.OnModelStepEnd != nil) {
				reqMsgs := buildRequestMessages(cfg.System, *msgs)
				toolN := 0
				if offerTools {
					toolN = len(cfg.Registry.OpenAITools())
				}
				stepMark := time.Now()
				if cfg.Lifecycle.OnModelStepStart != nil {
					cfg.Lifecycle.OnModelStepStart(ctx, step, toolN, reqMsgs)
				}
				if cfg.Lifecycle.OnModelStepEnd != nil {
					cfg.Lifecycle.OnModelStepEnd(ctx, step, ModelStepEndInfo{
						Model:                  cfg.Model,
						OK:                     false,
						DurationMs:             time.Since(stepMark).Milliseconds(),
						Err:                    ctx.Err(),
						BeforeRequestCancelled: true,
					})
				}
			}
			return ctx.Err()
		default:
		}

		reqMsgs := buildRequestMessages(cfg.System, *msgs)
		ApplyOutboundAssistantExtensionFields(reqMsgs)
		params := openai.ChatCompletionNewParams{
			Model:               cfg.Model,
			Messages:            reqMsgs,
			MaxCompletionTokens: openai.Int(cfg.MaxTokens),
			StreamOptions:       openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)},
		}
		if offerTools {
			params.Tools = cfg.Registry.OpenAITools()
			// Let the model batch tool calls; executor partitions by Registry.ConcurrencySafe (read parallel, write serial).
			params.ParallelToolCalls = openai.Bool(true)
		}

		slog.Info("loop.model.request",
			"step", step,
			"model", cfg.Model,
			"history_messages", len(reqMsgs),
			"tools", len(params.Tools),
			"tools_offered", offerTools,
			"max_completion_tokens", cfg.MaxTokens,
		)
		if cfg.Lifecycle != nil && cfg.Lifecycle.OnModelStepStart != nil {
			cfg.Lifecycle.OnModelStepStart(ctx, step, len(params.Tools), reqMsgs)
		}
		stepStart := time.Now()
		completion, err := model.CompleteWithTransport(ctx, cfg.Client, params, cfg.ChatTransport)
		if err != nil {
			slog.Error("loop.model.error", "step", step, "model", cfg.Model, "duration_ms", time.Since(stepStart).Milliseconds(), "err", err)
			if cfg.Lifecycle != nil && cfg.Lifecycle.OnModelStepEnd != nil {
				cfg.Lifecycle.OnModelStepEnd(ctx, step, ModelStepEndInfo{
					Model:      cfg.Model,
					OK:         false,
					DurationMs: time.Since(stepStart).Milliseconds(),
					Err:        err,
				})
			}
			return fmt.Errorf("model step %d: %w", step, err)
		}
		if len(completion.Choices) == 0 {
			emptyErr := fmt.Errorf("model step %d: empty choices", step)
			if cfg.Lifecycle != nil && cfg.Lifecycle.OnModelStepEnd != nil {
				cfg.Lifecycle.OnModelStepEnd(ctx, step, ModelStepEndInfo{
					Model:      cfg.Model,
					OK:         false,
					DurationMs: time.Since(stepStart).Milliseconds(),
					Err:        emptyErr,
				})
			}
			return emptyErr
		}

		choice := completion.Choices[0]
		*msgs = append(*msgs, assistantParamFromCompletion(choice.Message))
		if cfg.OutboundText != nil {
			if vis := assistantVisibleText(choice.Message); vis != "" {
				logOutboundEmit("text", cfg.OutboundText(ctx, vis))
			}
		}
		slog.Info("loop.model.response",
			"step", step,
			"finish_reason", choice.FinishReason,
			"duration_ms", time.Since(stepStart).Milliseconds(),
			"tool_calls", len(choice.Message.ToolCalls),
			"prompt_tokens", completion.Usage.PromptTokens,
			"completion_tokens", completion.Usage.CompletionTokens,
		)
		if cfg.Lifecycle != nil && cfg.Lifecycle.OnModelStepEnd != nil {
			endInfo := ModelStepEndInfo{
				Model:            cfg.Model,
				OK:               true,
				AssistantVisible: assistantVisibleText(choice.Message),
				FinishReason:     choice.FinishReason,
				ToolCallsCount:   len(choice.Message.ToolCalls),
				DurationMs:       time.Since(stepStart).Milliseconds(),
				PromptTokens:     completion.Usage.PromptTokens,
				CompletionTokens: completion.Usage.CompletionTokens,
				TotalTokens:      completion.Usage.TotalTokens,
			}
			endInfo.ToolCallsJSON = toolCallsJSONFromMessage(choice.Message)
			cfg.Lifecycle.OnModelStepEnd(ctx, step, endInfo)
		}

		cwd := ""
		var inbound bus.InboundMessage
		depth := 0
		if cfg.ToolContext != nil {
			cwd = cfg.ToolContext.CWD
			inbound = cfg.ToolContext.TurnInbound
			depth = cfg.ToolContext.SubagentDepth
		}
		wf := false
		var instr string
		if cfg.ToolContext != nil {
			wf = cfg.ToolContext.WorkspaceFlat
			instr = cfg.ToolContext.InstructionRoot
		}
		usageledger.MaybeRecord(usageledger.RecordParams{
			CWD:              cwd,
			SessionID:        cfg.SessionID,
			Model:            cfg.Model,
			Step:             step,
			SubagentDepth:    depth,
			PromptTokens:     completion.Usage.PromptTokens,
			CompletionTokens: completion.Usage.CompletionTokens,
			TotalTokens:      completion.Usage.TotalTokens,
			UsageJSON:        completion.Usage.RawJSON(),
			Inbound:          inbound,
			WorkspaceFlat:    wf,
			InstructionRoot:  instr,
		})

		// Decide tool rounds from the message payload, not only finish_reason: some gateways
		// leave finish_reason empty when returning tool_calls, which would otherwise skip tools
		// and end the turn with no assistant text (and no webchat outbound).
		calls := collectToolCalls(choice.Message)
		if len(calls) == 0 {
			recordSlimTranscript()
			return nil
		}
		if !offerTools {
			return fmt.Errorf("model step %d: tool_calls but tools were not offered (max_steps=%d)", step, maxSteps)
		}

		slog.Info("loop.tools.batch", "step", step, "n", len(calls), "names", toolCallNames(calls))

		toolMsgs, err := executeToolBatches(ctx, calls, cfg, step)
		if err != nil {
			return err
		}
		for _, tm := range toolMsgs {
			*msgs = append(*msgs, tm)
		}
		if cfg.ToolContext != nil {
			for _, um := range cfg.ToolContext.TakeDeferredUserMessagesAfterToolBatch() {
				*msgs = append(*msgs, um)
			}
		}
	}

	return fmt.Errorf("loop: internal: model loop fell through (max_steps=%d)", maxSteps)
}

func buildRequestMessages(system string, history []openai.ChatCompletionMessageParamUnion) []openai.ChatCompletionMessageParamUnion {
	if system == "" {
		return history
	}
	out := make([]openai.ChatCompletionMessageParamUnion, 0, len(history)+1)
	out = append(out, openai.SystemMessage(system))
	out = append(out, history...)
	return out
}

type toolCallInv struct {
	ID   string
	Name string
	Args json.RawMessage
}

func toolCallNames(calls []toolCallInv) []string {
	names := make([]string, len(calls))
	for i, c := range calls {
		names[i] = c.Name
	}
	return names
}

func collectToolCalls(msg openai.ChatCompletionMessage) []toolCallInv {
	var out []toolCallInv
	for _, tc := range msg.ToolCalls {
		out = append(out, toolCallInv{
			ID:   tc.ID,
			Name: tc.Function.Name,
			Args: json.RawMessage(tc.Function.Arguments),
		})
	}
	return out
}

func partitionToolCalls(calls []toolCallInv, reg *tools.Registry) [][]toolCallInv {
	var batches [][]toolCallInv
	for _, c := range calls {
		tool, ok := reg.Get(c.Name)
		safe := ok && tool.ConcurrencySafe()
		if safe && len(batches) > 0 {
			prev := batches[len(batches)-1]
			if len(prev) > 0 {
				pt, pok := reg.Get(prev[0].Name)
				if pok && pt.ConcurrencySafe() {
					batches[len(batches)-1] = append(batches[len(batches)-1], c)
					continue
				}
			}
		}
		batches = append(batches, []toolCallInv{c})
	}
	return batches
}

func executeToolBatches(ctx context.Context, calls []toolCallInv, cfg Config, modelStep int) ([]openai.ChatCompletionMessageParamUnion, error) {
	batches := partitionToolCalls(calls, cfg.Registry)
	results := make([]openai.ChatCompletionMessageParamUnion, len(calls))
	index := 0
	for _, batch := range batches {
		start := index
		if len(batch) == 1 || !concurrentBatch(batch, cfg.Registry) {
			for _, c := range batch {
				msg, err := runOneTool(ctx, c, cfg, modelStep)
				if err != nil {
					return nil, err
				}
				results[start] = msg
				start++
			}
			index = start
			continue
		}
		g, gctx := errgroup.WithContext(ctx)
		for i, c := range batch {
			i, c := i, c
			g.Go(func() error {
				msg, err := runOneTool(gctx, c, cfg, modelStep)
				if err != nil {
					return err
				}
				results[start+i] = msg
				return nil
			})
		}
		if err := g.Wait(); err != nil {
			return nil, err
		}
		index += len(batch)
	}
	return results, nil
}

func concurrentBatch(batch []toolCallInv, reg *tools.Registry) bool {
	for _, c := range batch {
		t, ok := reg.Get(c.Name)
		if !ok || !t.ConcurrencySafe() {
			return false
		}
	}
	return true
}

func runOneTool(ctx context.Context, c toolCallInv, cfg Config, modelStep int) (openai.ChatCompletionMessageParamUnion, error) {
	t0 := time.Now()
	toolEnd := func(ok bool) {
		_ = ok
	}
	emitTool := func(ok bool, errText, out string) {
		ent := ToolTraceEntry{
			Step:        modelStep,
			ToolUseID:   c.ID,
			Name:        c.Name,
			OK:          ok,
			Err:         truncateRunes(errText, toolTraceErrMaxRunes),
			ArgsPreview: previewArgsJSON(c.Args),
			OutPreview:  previewToolOut(out),
			DurationMs:  time.Since(t0).Milliseconds(),
		}
		if cfg.ToolTrace != nil {
			cfg.ToolTrace.Add(ent)
		}
		if cfg.OnToolLogged != nil {
			cfg.OnToolLogged(ent)
		}
	}
	if cfg.CanUseTool != nil {
		allow, reason := cfg.CanUseTool(ctx, c.Name, c.Args, cfg.ToolContext)
		if !allow {
			slog.Warn("loop.tool.denied", "tool", c.Name, "tool_use_id", c.ID, "reason", reason)
			toolEnd(false)
			emitTool(false, reason, "")
			return openai.ToolMessage("denied: "+reason, c.ID), nil
		}
	}
	tool, ok := cfg.Registry.Get(c.Name)
	if !ok {
		slog.Warn("loop.tool.unknown", "tool", c.Name, "tool_use_id", c.ID)
		toolEnd(false)
		msg := fmt.Sprintf("unknown tool %q", c.Name)
		emitTool(false, msg, "")
		return openai.ToolMessage(msg, c.ID), nil
	}
	if cfg.Lifecycle != nil && cfg.Lifecycle.OnToolStart != nil {
		cfg.Lifecycle.OnToolStart(ctx, modelStep, c.ID, c.Name, previewArgsJSON(c.Args))
	}
	out, err := tool.Execute(ctx, c.Args, cfg.ToolContext)
	if err != nil {
		slog.Warn("loop.tool.error", "tool", c.Name, "tool_use_id", c.ID, "duration_ms", time.Since(t0).Milliseconds(), "err", err)
		toolEnd(false)
		errText := err.Error()
		emitTool(false, errText, "")
		return openai.ToolMessage(errText, c.ID), nil
	}
	slog.Debug("loop.tool.ok", "tool", c.Name, "tool_use_id", c.ID, "duration_ms", time.Since(t0).Milliseconds(), "out_bytes", len(out))
	toolEnd(true)
	emitTool(true, "", out)
	return openai.ToolMessage(out, c.ID), nil
}

func toolCallsJSONFromMessage(msg openai.ChatCompletionMessage) string {
	if len(msg.ToolCalls) == 0 {
		return ""
	}
	type item struct {
		ID   string          `json:"id"`
		Name string          `json:"name"`
		Args json.RawMessage `json:"arguments"`
	}
	items := make([]item, 0, len(msg.ToolCalls))
	for _, tc := range msg.ToolCalls {
		args := json.RawMessage(tc.Function.Arguments)
		if len(args) == 0 {
			args = json.RawMessage("null")
		}
		items = append(items, item{ID: tc.ID, Name: tc.Function.Name, Args: args})
	}
	b, err := json.Marshal(items)
	if err != nil {
		return ""
	}
	return string(b)
}

// TranscriptJSON is a serializable view of the conversation (API-shaped messages).
type TranscriptJSON struct {
	Messages []json.RawMessage `json:"messages"`
}

// MarshalMessages encodes message params to JSON for persistence.
func MarshalMessages(msgs []openai.ChatCompletionMessageParamUnion) ([]byte, error) {
	raw := make([]json.RawMessage, len(msgs))
	for i, m := range msgs {
		b, err := json.Marshal(m)
		if err != nil {
			return nil, err
		}
		raw[i] = b
	}
	return json.MarshalIndent(TranscriptJSON{Messages: raw}, "", "  ")
}

// UnmarshalMessages decodes transcript bytes into message slice.
func UnmarshalMessages(data []byte) ([]openai.ChatCompletionMessageParamUnion, error) {
	var wrap TranscriptJSON
	if err := json.Unmarshal(data, &wrap); err != nil {
		return nil, err
	}
	out := make([]openai.ChatCompletionMessageParamUnion, len(wrap.Messages))
	for i, r := range wrap.Messages {
		if err := json.Unmarshal(r, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}
