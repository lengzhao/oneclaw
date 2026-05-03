package wfexec

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"github.com/lengzhao/oneclaw/adkhost"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/observe"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/workflow"
)

func handleAgent(rtx *engine.RuntimeContext) error {
	subType := workflow.AgentTypeParam(rtx.CurrentParams)
	if subType == "" {
		return fmt.Errorf("wfexec: agent: missing params.agent_type")
	}
	if rtx.Catalog == nil || rtx.Cfg == nil {
		return fmt.Errorf("wfexec: agent: Catalog and Cfg must be set on RuntimeContext")
	}
	if strings.TrimSpace(rtx.EffectiveUserDataRoot()) == "" || strings.TrimSpace(rtx.EffectiveInstructionRoot()) == "" {
		return fmt.Errorf("wfexec: agent: UserDataRoot and InstructionRoot must be set")
	}
	sub := rtx.Catalog.Get(subType)
	if sub == nil {
		return fmt.Errorf("wfexec: agent: unknown agent_type %q", subType)
	}

	bundle, err := preturn.Build(rtx.EffectiveUserDataRoot(), rtx.EffectiveInstructionRoot(), sub, preturn.DefaultBudget())
	if err != nil {
		return err
	}
	execReg, err := toolRegistryForBundle(rtx.EffectiveWorkspacePath(), bundle)
	if err != nil {
		return err
	}

	profID := strings.TrimSpace(sub.Model)
	if profID == "" {
		profID = rtx.EffectiveProfileID()
	}
	prof, err := config.ResolveModelProfile(rtx.Cfg, profID)
	if err != nil {
		return fmt.Errorf("wfexec: agent %q: %w", subType, err)
	}

	useMock := rtx.EffectiveUseMock() || strings.EqualFold(prof.Provider, "mock")
	cm, err := chatModelForProfile(rtx.GoCtx, prof, useMock)
	if err != nil {
		return fmt.Errorf("wfexec: agent %q: %w", subType, err)
	}

	desc := sub.Description
	if desc == "" {
		desc = sub.Name
	}
	maxIt := rtx.Cfg.Runtime.MaxAgentIterations
	if maxIt <= 0 {
		maxIt = 100
	}

	runAgent, err := adkhost.NewChatModelAgent(rtx.GoCtx, cm, execReg, adkhost.AgentOptions{
		Name:          sub.AgentType,
		Description:   desc,
		Instruction:   bundle.Instruction,
		MaxIterations: maxIt,
		Handlers:      []adk.ChatModelAgentMiddleware{observe.NewChatModelLogMiddleware()},
	})
	if err != nil {
		return err
	}

	input := &adk.AgentInput{
		Messages: []adk.Message{schema.UserMessage(agentTurnUserContent(rtx))},
	}
	iter := runAgent.Run(rtx.GoCtx, input)
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			return fmt.Errorf("wfexec: agent %q: %w", subType, ev.Err)
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil && ev.Output.MessageOutput.Message != nil {
			msg := ev.Output.MessageOutput.Message
			c := strings.TrimSpace(msg.Content)
			if c != "" && rtx.Stdout != nil {
				fmt.Fprintln(rtx.Stdout, c)
			}
		}
	}
	return nil
}

func agentTurnUserContent(rtx *engine.RuntimeContext) string {
	var b strings.Builder
	b.WriteString("Context for this agent run:\n\nUser message:\n")
	b.WriteString(strings.TrimSpace(rtx.EffectiveUserPrompt()))
	if a := strings.TrimSpace(rtx.Assistant); a != "" {
		b.WriteString("\n\nMain agent assistant reply:\n")
		b.WriteString(a)
	}
	return b.String()
}

func toolRegistryForBundle(ws string, bundle *preturn.Bundle) (*tools.Registry, error) {
	baseReg := tools.NewRegistry(ws)
	if err := tools.RegisterBuiltins(baseReg); err != nil {
		return nil, err
	}
	chosen := baseReg.All()
	var err error
	if len(bundle.ToolAllowlist) > 0 {
		chosen, err = baseReg.FilterByNames(bundle.ToolAllowlist)
		if err != nil {
			return nil, err
		}
	}
	execReg := tools.NewRegistry(ws)
	for _, t := range chosen {
		if err := execReg.Register(t); err != nil {
			return nil, err
		}
	}
	return execReg, nil
}

func chatModelForProfile(ctx context.Context, prof *config.ModelProfile, useMock bool) (model.ToolCallingChatModel, error) {
	if useMock {
		return adkhost.NewStubChatModel("Hello from oneclaw stub model."), nil
	}
	return adkhost.NewOpenAIChatModel(ctx, prof)
}
