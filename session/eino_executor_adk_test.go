package session

import (
	"context"
	"encoding/json"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/openai/openai-go"
)

func TestADKEinoExecutor_ValidateInputs(t *testing.T) {
	exec := newADKEinoExecutor()
	err := exec.Execute(context.Background(), loop.Config{}, bus.InboundMessage{}, []tools.EinoBinding{})
	if err == nil {
		t.Fatal("expected error for nil Messages/Registry")
	}
}

func TestADKEinoExecutor_ErrorsWithoutAPIKey(t *testing.T) {
	exec := newADKEinoExecutor()
	msgs := make([]openai.ChatCompletionMessageParamUnion, 0)
	cfg := loop.Config{
		Messages: &msgs,
		Registry: tools.NewRegistry(),
	}
	err := exec.Execute(context.Background(), cfg, bus.InboundMessage{}, []tools.EinoBinding{})
	if err == nil {
		t.Fatal("expected error without API key")
	}
}

func TestBuildEinoTools_InfoAndInvoke(t *testing.T) {
	called := false
	var gotArgs string
	bindings := []tools.EinoBinding{
		{
			Name:           "echo_tool",
			Description:    "echo description",
			ParametersJSON: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string"}}}`),
			Execute: func(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
				called = true
				gotArgs = string(input)
				return "ok:" + string(input), nil
			},
		},
	}
	etools := buildEinoTools(bindings, &toolctx.Context{})
	if len(etools) != 1 {
		t.Fatalf("tools len: got %d want 1", len(etools))
	}
	info, err := etools[0].Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error: %v", err)
	}
	if info.Name != "echo_tool" || info.Desc != "echo description" {
		t.Fatalf("unexpected tool info: %+v", info)
	}
	inv, ok := etools[0].(einotool.InvokableTool)
	if !ok {
		t.Fatal("tool should implement InvokableTool")
	}
	out, err := inv.InvokableRun(context.Background(), `{"text":"hi"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error: %v", err)
	}
	if !called {
		t.Fatal("expected binding execute to be called")
	}
	if gotArgs != `{"text":"hi"}` {
		t.Fatalf("args: got %s", gotArgs)
	}
	if out != `ok:{"text":"hi"}` {
		t.Fatalf("output: got %s", out)
	}
}

