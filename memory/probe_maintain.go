package memory

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/lengzhao/oneclaw/model"
	"github.com/openai/openai-go"
)

// ProbeMaintenanceModels sends a minimal chat completion for each distinct resolved maintenance model
// (post_turn vs scheduled YAML overrides). Use from CLI to verify gateway routing before long runs.
func ProbeMaintenanceModels(ctx context.Context, client *openai.Client, mainChatModel string) error {
	if client == nil {
		return fmt.Errorf("nil openai client")
	}
	seen := map[string]struct{}{}
	for _, scheduled := range []bool{false, true} {
		m, dedicated := ResolveMaintenanceModel(mainChatModel, scheduled)
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		mode := "post_turn"
		if scheduled {
			mode = "scheduled"
		}
		params := openai.ChatCompletionNewParams{
			Model: m,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage("ping"),
			},
			MaxCompletionTokens: openai.Int(8),
		}
		_, err := model.Complete(ctx, client, params)
		if err != nil {
			return fmt.Errorf("%s maintenance model %q (dedicated_override=%v): %w", mode, m, dedicated, err)
		}
		slog.Info("memory.maintain.probe_ok", "mode", mode, "model", m, "dedicated_model", dedicated)
	}
	return nil
}
