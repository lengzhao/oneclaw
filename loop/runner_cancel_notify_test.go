package loop

import (
	"context"
	"errors"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

func TestRunTurnCancelBeforeModelEmitsLifecycle(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var starts, ends int
	var endInfo ModelStepEndInfo
	msgs := []openai.ChatCompletionMessageParamUnion{}
	cfg := Config{
		Client:    &openai.Client{},
		Model:     "gpt-4o-mini",
		MaxSteps:  8,
		Messages:  &msgs,
		Registry:  tools.NewRegistry(),
		Lifecycle: &LifecycleCallbacks{
			OnModelStepStart: func(ctx context.Context, step, toolN int, reqMsgs []openai.ChatCompletionMessageParamUnion) {
				starts++
				if step != 0 {
					t.Fatalf("step %d", step)
				}
				_ = toolN
				_ = reqMsgs
			},
			OnModelStepEnd: func(ctx context.Context, step int, end ModelStepEndInfo) {
				ends++
				endInfo = end
			},
		},
	}
	err := RunTurn(ctx, cfg, bus.InboundMessage{Content: "hi"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v", err)
	}
	if starts != 1 || ends != 1 {
		t.Fatalf("starts=%d ends=%d", starts, ends)
	}
	if endInfo.OK || !endInfo.BeforeRequestCancelled || !errors.Is(endInfo.Err, context.Canceled) {
		t.Fatalf("%+v", endInfo)
	}
}
