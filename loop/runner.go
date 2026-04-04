package loop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/model"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
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
	*msgs = append(*msgs, openai.UserMessage(in.Text))

	for step := 0; step < cfg.MaxSteps; step++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		reqMsgs := buildRequestMessages(cfg.System, *msgs)
		params := openai.ChatCompletionNewParams{
			Model:               cfg.Model,
			Messages:            reqMsgs,
			Tools:               cfg.Registry.OpenAITools(),
			MaxTokens:           openai.Int(cfg.MaxTokens),
			ParallelToolCalls:   openai.Bool(false),
			StreamOptions:       openai.ChatCompletionStreamOptionsParam{IncludeUsage: openai.Bool(true)},
		}

		slog.Info("loop.model.request",
			"step", step,
			"model", cfg.Model,
			"history_messages", len(reqMsgs),
			"tools", len(params.Tools),
			"max_tokens", cfg.MaxTokens,
		)
		stepStart := time.Now()
		completion, err := model.Complete(ctx, cfg.Client, params)
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
			return nil
		}

		calls := collectToolCalls(choice.Message)
		if len(calls) == 0 {
			return nil
		}
		slog.Info("loop.tools.batch", "step", step, "n", len(calls), "names", toolCallNames(calls))

		if cfg.Outbound != nil {
			for _, c := range calls {
				logOutboundEmit("tool_start", cfg.Outbound.ToolStart(ctx, c.Name))
			}
		}

		toolMsgs, err := executeToolBatches(ctx, calls, cfg)
		if err != nil {
			return err
		}
		for _, tm := range toolMsgs {
			*msgs = append(*msgs, tm)
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

func executeToolBatches(ctx context.Context, calls []toolCallInv, cfg Config) ([]openai.ChatCompletionMessageParamUnion, error) {
	batches := partitionToolCalls(calls, cfg.Registry)
	results := make([]openai.ChatCompletionMessageParamUnion, len(calls))
	index := 0
	for _, batch := range batches {
		start := index
		if len(batch) == 1 || !concurrentBatch(batch, cfg.Registry) {
			for _, c := range batch {
				msg, err := runOneTool(ctx, c, cfg)
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
				msg, err := runOneTool(gctx, c, cfg)
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

func runOneTool(ctx context.Context, c toolCallInv, cfg Config) (openai.ChatCompletionMessageParamUnion, error) {
	t0 := time.Now()
	toolEnd := func(ok bool) {
		if cfg.Outbound != nil {
			logOutboundEmit("tool_end", cfg.Outbound.ToolEnd(ctx, c.Name, ok))
		}
	}
	if cfg.CanUseTool != nil {
		allow, reason := cfg.CanUseTool(ctx, c.Name, c.Args, cfg.ToolContext)
		if !allow {
			slog.Warn("loop.tool.denied", "tool", c.Name, "tool_use_id", c.ID, "reason", reason)
			toolEnd(false)
			return openai.ToolMessage("denied: "+reason, c.ID), nil
		}
	}
	tool, ok := cfg.Registry.Get(c.Name)
	if !ok {
		slog.Warn("loop.tool.unknown", "tool", c.Name, "tool_use_id", c.ID)
		toolEnd(false)
		return openai.ToolMessage(fmt.Sprintf("unknown tool %q", c.Name), c.ID), nil
	}
	out, err := tool.Execute(ctx, c.Args, cfg.ToolContext)
	if err != nil {
		slog.Warn("loop.tool.error", "tool", c.Name, "tool_use_id", c.ID, "duration_ms", time.Since(t0).Milliseconds(), "err", err)
		toolEnd(false)
		return openai.ToolMessage(err.Error(), c.ID), nil
	}
	slog.Debug("loop.tool.ok", "tool", c.Name, "tool_use_id", c.ID, "duration_ms", time.Since(t0).Milliseconds(), "out_bytes", len(out))
	toolEnd(true)
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
