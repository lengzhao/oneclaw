package model

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/openai/openai-go"
)

// Complete calls the chat API. Behavior depends on ONCLAW_CHAT_TRANSPORT (see transport.go).
func Complete(ctx context.Context, client *openai.Client, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	switch chatTransportFromEnv() {
	case transportNonStream:
		t0 := time.Now()
		res, err := completeNonStream(ctx, client, params)
		if err == nil {
			slog.Info("openai.chat.ok",
				"transport", "non_stream",
				"model", params.Model,
				"duration_ms", time.Since(t0).Milliseconds(),
			)
		}
		return res, err
	case transportStream:
		return completeStreamOnly(ctx, client, params)
	default:
		return completeAuto(ctx, client, params)
	}
}

func completeAuto(ctx context.Context, client *openai.Client, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	t0 := time.Now()
	res, err := streamCompletion(ctx, client, params)
	if err == nil {
		slog.Debug("openai.chat.ok",
			"transport", "stream",
			"model", params.Model,
			"duration_ms", time.Since(t0).Milliseconds(),
		)
		return res, nil
	}

	// Expected on some OpenAI-compatible gateways; fallback is normal — avoid WARN.
	slog.Debug("openai.chat.stream_unavailable",
		"model", params.Model,
		"duration_ms", time.Since(t0).Milliseconds(),
		"err", err,
		"hint", "set ONCLAW_CHAT_TRANSPORT=non_stream to skip streaming and save latency",
	)

	t1 := time.Now()
	res, err2 := completeNonStream(ctx, client, params)
	if err2 != nil {
		return nil, fmt.Errorf("stream: %w; non-stream: %v", err, err2)
	}
	slog.Info("openai.chat.ok",
		"transport", "non_stream",
		"model", params.Model,
		"duration_ms", time.Since(t1).Milliseconds(),
		"stream_fallback", true,
	)
	return res, nil
}

func completeStreamOnly(ctx context.Context, client *openai.Client, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	t0 := time.Now()
	res, err := streamCompletion(ctx, client, params)
	if err != nil {
		slog.Error("openai.chat.stream_failed",
			"model", params.Model,
			"duration_ms", time.Since(t0).Milliseconds(),
			"err", err,
		)
		return nil, err
	}
	slog.Debug("openai.chat.ok",
		"transport", "stream",
		"model", params.Model,
		"duration_ms", time.Since(t0).Milliseconds(),
	)
	return res, nil
}

func completeNonStream(ctx context.Context, client *openai.Client, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	t0 := time.Now()
	p := params
	p.StreamOptions = openai.ChatCompletionStreamOptionsParam{}
	res, err := client.Chat.Completions.New(ctx, p)
	if err != nil {
		slog.Error("openai.chat.nonstream_failed",
			"model", params.Model,
			"duration_ms", time.Since(t0).Milliseconds(),
			"err", err,
		)
		return nil, err
	}
	return res, nil
}

func streamCompletion(ctx context.Context, client *openai.Client, params openai.ChatCompletionNewParams) (*openai.ChatCompletion, error) {
	stream := client.Chat.Completions.NewStreaming(ctx, params)
	defer stream.Close()

	var acc openai.ChatCompletionAccumulator
	nChunk := 0
	for stream.Next() {
		nChunk++
		chunk := stream.Current()
		if slog.Default().Enabled(ctx, slog.LevelDebug) {
			slog.Debug("openai.chat.chunk",
				"n", nChunk,
				"choices", len(chunk.Choices),
			)
		}
		if !acc.AddChunk(chunk) {
			return nil, fmt.Errorf("stream accumulate: chunk mismatch (after %d chunks)", nChunk)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	if nChunk == 0 {
		return nil, fmt.Errorf("empty stream (no chunks)")
	}
	return &acc.ChatCompletion, nil
}
