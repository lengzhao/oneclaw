package wfexec

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/toolhost"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/workflow"
)

func handleAgentTask(_ context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	rtx := env.Runtime
	subType := strings.TrimSpace(env.Node.AgentType)
	if subType == "" {
		subType = workflow.AgentTypeParam(env.Node.Params)
	}
	if subType == "" {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: agent_task: missing agent_type")
	}
	reply, err := executeAgentTask(rtx, subType, strings.TrimSpace(in.Text))
	if err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	return workflow.WorkflowNodeResult{Text: reply}, nil
}

func executeAgentTask(rtx *engine.RuntimeContext, subType string, prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		p, err := BuildSubagentUserPrompt(rtx)
		if err != nil {
			return "", err
		}
		prompt = p
	}
	if rtx.Catalog == nil || rtx.Cfg == nil {
		return "", fmt.Errorf("wfexec: agent_task: Catalog and Cfg must be set on RuntimeContext")
	}
	if strings.TrimSpace(rtx.EffectiveUserDataRoot()) == "" || strings.TrimSpace(rtx.EffectiveInstructionRoot()) == "" {
		return "", fmt.Errorf("wfexec: agent_task: UserDataRoot and InstructionRoot must be set")
	}
	sub := rtx.Catalog.Get(subType)
	if sub == nil {
		return "", fmt.Errorf("wfexec: agent_task: unknown agent_type %q", subType)
	}

	var parentReg toolhost.Registry = rtx.ToolRegistry
	if parentReg == nil {
		r := tools.NewRegistry(rtx.EffectiveWorkspacePath())
		if err := tools.RegisterBuiltinsForConfig(r, rtx.Cfg); err != nil {
			return "", err
		}
		parentReg = r
	}

	corr := strings.TrimSpace(rtx.CorrelationID)
	if corr == "" {
		corr = subagent.NewCorrelationID()
	}
	agentID := strings.TrimSpace(rtx.Turn.AgentID)
	if agentID == "" && rtx.Agent != nil {
		agentID = rtx.Agent.AgentType
	}
	deps := &subagent.RunAgentDeps{
		Turn: subagent.TurnBinding{
			SessionSegment:  rtx.EffectiveSessionSegment(),
			InboundClientID: "",
			AgentID:         agentID,
			ReplyMeta:       maps.Clone(rtx.Turn.ReplyMeta),
		},
		HostAgentID:     agentID,
		Catalog:         rtx.Catalog,
		Cfg:             rtx.Cfg,
		UserDataRoot:    rtx.EffectiveUserDataRoot(),
		InstructionRoot: rtx.EffectiveInstructionRoot(),
		SessionRoot:     rtx.EffectiveSessionRoot(),
		ParentWorkspace: rtx.EffectiveWorkspacePath(),
		ProfileID:       rtx.EffectiveProfileID(),
		ModelName:       rtx.EffectiveModelName(),
		UseMock:         rtx.EffectiveUseMock(),
		Stdout:          rtx.Stdout,
		OnSubAgentChunk: rtx.OnSubAgentAssistantChunk,
		CorrelationID:   corr,
		ParentRegistry:  parentReg,
		DelegationDepth: rtx.DelegationDepth,
	}
	reply, err := subagent.ExecuteSubAgentTurn(rtx.GoCtx, deps, sub, prompt)
	return reply, err
}
