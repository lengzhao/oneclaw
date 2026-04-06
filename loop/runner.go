package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/model"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
	"golang.org/x/sync/errgroup"
)

// Config drives one user turn (may include multiple model round-trips for tools).
type Config struct {
	Client      *openai.Client
	Model       string
	System      string
	MaxTokens   int64
	MaxSteps    int // model calls per SubmitUser turn
	Messages    *[]openai.ChatCompletionMessageParamUnion
	Registry    *tools.Registry
	ToolContext *toolctx.Context
	CanUseTool  tools.CanUseTool
	// Outbound emits text/tool/done events; nil disables outbound for this turn.
	Outbound *routing.Emitter
	// MemoryAgentMd / MemoryRecall are optional extra user messages before the real user turn (phase B).
	MemoryAgentMd string
	MemoryRecall  string
	// Budget trims transcript before each model call when Enabled (from budget.FromEnv).
	Budget budget.Global
	// ChatTransport overrides ONCLAW_CHAT_TRANSPORT when non-empty (e.g. from config file).
	ChatTransport string
	// ToolTrace, when non-nil, records slim per-tool rows for this RunTurn only (not sent to the model).
	ToolTrace *ToolTraceSink
	// OnToolLogged, when non-nil, called synchronously after each tool completes (e.g. append-only JSONL).
	OnToolLogged func(ToolTraceEntry)
	// SlimTranscript, if non-nil, called once when RunTurn finishes successfully with the final
	// assistant-visible reply (after any tool rounds). User turn is recorded before the model loop
	// by the caller (Claude Code–style: recordTranscript before query). Not called on error or abort.
	SlimTranscript func(assistantText string)
}

func logOutboundEmit(op string, err error) {
	if err != nil {
		slog.Warn("loop.outbound.emit_failed", "op", op, "err", err)
	}
}

// RunTurn appends a user message from in.Text, then runs model ↔ tool until stop or limits.
func RunTurn(ctx context.Context, cfg Config, in routing.Inbound) (err error) {
	if cfg.Messages == nil {
		return fmt.Errorf("loop: Config.Messages is nil")
	}
	if cfg.ToolContext != nil {
		routing.MergeNonEmptyRouting(&cfg.ToolContext.TurnInbound, in)
	}
	defer func() {
		emit := cfg.Outbound
		if emit == nil {
			return
		}
		bg := context.Background()
		if err != nil {
			msg := err.Error()
			if errors.Is(err, context.Canceled) {
				msg = "aborted"
			}
			logOutboundEmit("done", emit.Done(bg, false, msg))
			return
		}
		logOutboundEmit("done", emit.Done(bg, true, ""))
	}()

	msgs := cfg.Messages
	recordSlimTranscript := func() {
		if cfg.SlimTranscript == nil {
			return
		}
		cfg.SlimTranscript(LastAssistantDisplay(*msgs))
	}
	if s := strings.TrimSpace(cfg.MemoryAgentMd); s != "" {
		*msgs = append(*msgs, openai.UserMessage(s))
	}
	if s := strings.TrimSpace(cfg.MemoryRecall); s != "" {
		*msgs = append(*msgs, openai.UserMessage(s))
	}
	*msgs = append(*msgs, openai.UserMessage(in.Text))

	for step := 0; step < cfg.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		ApplyHistoryBudget(cfg.Budget, cfg.System, msgs)
		reqMsgs := buildRequestMessages(cfg.System, *msgs)
		params := openai.ChatCompletionNewParams{
			Model:               cfg.Model,
			Messages:            reqMsgs,
			Tools:               cfg.Registry.OpenAITools(),
			MaxCompletionTokens: openai.Int(cfg.MaxTokens),
			// Let the model batch tool calls; executor partitions by Registry.ConcurrencySafe (read parallel, write serial).
			ParallelToolCalls: openai.Bool(true),
			StreamOptions:     openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)},
		}

		slog.Info("loop.model.request",
			"step", step,
			"model", cfg.Model,
			"history_messages", len(reqMsgs),
			"tools", len(params.Tools),
			"max_completion_tokens", cfg.MaxTokens,
		)
		stepStart := time.Now()
		completion, err := model.CompleteWithTransport(ctx, cfg.Client, params, cfg.ChatTransport)
		if err != nil {
			slog.Error("loop.model.error", "step", step, "model", cfg.Model, "duration_ms", time.Since(stepStart).Milliseconds(), "err", err)
			return fmt.Errorf("model step %d: %w", step, err)
		}
		if len(completion.Choices) == 0 {
			return fmt.Errorf("model step %d: empty choices", step)
		}

		choice := completion.Choices[0]
		*msgs = append(*msgs, choice.Message.ToParam())
		if cfg.Outbound != nil {
			if vis := assistantVisibleText(choice.Message); vis != "" {
				logOutboundEmit("text", cfg.Outbound.Text(ctx, vis))
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

		if choice.FinishReason != "tool_calls" {
			recordSlimTranscript()
			return nil
		}

		calls := collectToolCalls(choice.Message)
		if len(calls) == 0 {
			recordSlimTranscript()
			return nil
		}
		slog.Info("loop.tools.batch", "step", step, "n", len(calls), "names", toolCallNames(calls))

		if cfg.Outbound != nil {
			for _, c := range calls {
				logOutboundEmit("tool_start", cfg.Outbound.ToolStart(ctx, c.Name))
			}
		}

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

	return fmt.Errorf("max model steps (%d) exceeded", cfg.MaxSteps)
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
		if cfg.Outbound != nil {
			logOutboundEmit("tool_end", cfg.Outbound.ToolEnd(ctx, c.Name, ok))
		}
	}
	emitTool := func(ok bool, errText, out string) {
		ent := ToolTraceEntry{
			Step:        modelStep,
			Name:        c.Name,
			OK:          ok,
			Err:         truncateRunes(errText, 200),
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
