package session

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/prompt"
	"github.com/cloudwego/eino/schema"
	"github.com/lengzhao/oneclaw/loop"
)

// loopMessagesGenModelInput builds ADK model messages: system instruction plus transcript (*schema.Message).
// Runner.Run is called with nil messages; the payload comes entirely from cfg.Messages here.
func loopMessagesGenModelInput(cfg *loop.Config) adk.GenModelInput {
	return func(ctx context.Context, instruction string, input *adk.AgentInput) ([]adk.Message, error) {
		return buildADKMessagesFromLoop(ctx, instruction, cfg.Messages)
	}
}

func buildADKMessagesFromLoop(ctx context.Context, instruction string, msgs *[]*schema.Message) ([]adk.Message, error) {
	out := make([]adk.Message, 0)
	if s := strings.TrimSpace(instruction); s != "" {
		sp := schema.SystemMessage(s)
		vs := adk.GetSessionValues(ctx)
		if len(vs) > 0 {
			ct := prompt.FromMessages(schema.FString, sp)
			ms, err := ct.Format(ctx, vs)
			if err != nil {
				return nil, fmt.Errorf("session: eino instruction format: %w", err)
			}
			if len(ms) == 0 {
				return nil, fmt.Errorf("session: eino instruction format: empty result")
			}
			sp = ms[0]
		}
		out = append(out, sp)
	}
	if msgs == nil {
		return out, nil
	}
	for _, m := range *msgs {
		if m != nil {
			out = append(out, m)
		}
	}
	return out, nil
}
