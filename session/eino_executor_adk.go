package session

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	einoschema "github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
)

// adkEinoExecutor runs the Eino ADK ChatModelAgent. OpenAI API key is required
// ([loop.Config.EinoOpenAIAPIKey]).
type adkEinoExecutor struct{}

func newADKEinoExecutor() EinoExecutor {
	return adkEinoExecutor{}
}

func (e adkEinoExecutor) Execute(ctx context.Context, cfg loop.Config, in bus.InboundMessage, bindings []tools.EinoBinding) error {
	if cfg.Messages == nil {
		return fmt.Errorf("session: eino executor requires non-nil message history")
	}
	if cfg.Registry == nil {
		return fmt.Errorf("session: eino executor requires non-nil tool registry")
	}
	if bindings == nil {
		return fmt.Errorf("session: eino executor requires non-nil bindings")
	}
	if strings.TrimSpace(cfg.EinoOpenAIAPIKey) == "" {
		return fmt.Errorf("session: eino requires openai api key (config openai.api_key / Engine.EinoOpenAIAPIKey)")
	}

	userLine := strings.TrimSpace(cfg.UserLine)
	if userLine == "" {
		userLine = strings.TrimSpace(in.Content)
	}
	loop.AppendTurnUserMessages(cfg.Messages, cfg.MemoryAgentMd, cfg.InboundMeta, cfg.InboundAttachmentChunks, userLine)
	loop.ApplyHistoryBudget(cfg.Budget, strings.TrimSpace(cfg.System), cfg.Messages)

	cm, err := e.buildChatModel(ctx, cfg)
	if err != nil {
		return err
	}
	var handlers []adk.ChatModelAgentMiddleware
	if h := maybeBeforeModelInjectHandler(&cfg); h != nil {
		handlers = append(handlers, h)
	}
	agent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "oneclaw_eino_runtime",
		Description: "oneclaw runtime via eino adk",
		Instruction: strings.TrimSpace(cfg.System),
		Model:       cm,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: buildEinoTools(bindings, cfg.ToolContext),
			},
		},
		Handlers:      handlers,
		GenModelInput: loopMessagesGenModelInput(&cfg),
		MaxIterations: max(1, cfg.TurnMaxSteps),
	})
	if err != nil {
		return fmt.Errorf("session: eino create agent: %w", err)
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := runner.Run(ctx, nil)
	reply := ""
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			return fmt.Errorf("session: eino runner: %w", ev.Err)
		}
		if ev.Output == nil || ev.Output.MessageOutput == nil {
			continue
		}
		msg, err := ev.Output.MessageOutput.GetMessage()
		if err != nil || msg == nil {
			continue
		}
		if strings.TrimSpace(msg.Content) != "" {
			reply = msg.Content
		}
	}
	if strings.TrimSpace(reply) == "" {
		return fmt.Errorf("session: eino empty reply")
	}
	*cfg.Messages = append(*cfg.Messages, schema.AssistantMessage(reply, nil))
	if cfg.OutboundText != nil {
		_ = cfg.OutboundText(ctx, reply)
	}
	if cfg.SlimTranscript != nil {
		cfg.SlimTranscript(reply)
	}
	return nil
}

func (e adkEinoExecutor) buildChatModel(ctx context.Context, cfg loop.Config) (*einoopenai.ChatModel, error) {
	apiKey := strings.TrimSpace(cfg.EinoOpenAIAPIKey)
	if apiKey == "" {
		return nil, fmt.Errorf("session: eino api key is empty")
	}
	conf := &einoopenai.ChatModelConfig{
		APIKey: apiKey,
		Model:  cfg.Model,
	}
	if u := strings.TrimSpace(cfg.EinoOpenAIBaseURL); u != "" {
		conf.BaseURL = u
	}
	cm, err := einoopenai.NewChatModel(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("session: eino chat model: %w", err)
	}
	return cm, nil
}

type oneclawEinoTool struct {
	binding tools.EinoBinding
	tctx    *toolctx.Context
}

func (t oneclawEinoTool) Info(ctx context.Context) (*einoschema.ToolInfo, error) {
	js := &jsonschema.Schema{}
	if len(t.binding.ParametersJSON) > 0 {
		if err := json.Unmarshal(t.binding.ParametersJSON, js); err != nil {
			return nil, fmt.Errorf("session: eino tool schema %s: %w", t.binding.Name, err)
		}
	}
	return &einoschema.ToolInfo{
		Name:        t.binding.Name,
		Desc:        t.binding.Description,
		ParamsOneOf: einoschema.NewParamsOneOfByJSONSchema(js),
	}, nil
}

func (t oneclawEinoTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...einotool.Option) (string, error) {
	return t.binding.Execute(ctx, json.RawMessage(argumentsInJSON), t.tctx)
}

func buildEinoTools(bindings []tools.EinoBinding, tctx *toolctx.Context) []einotool.BaseTool {
	out := make([]einotool.BaseTool, 0, len(bindings))
	for _, b := range bindings {
		out = append(out, oneclawEinoTool{binding: b, tctx: tctx})
	}
	return out
}
