package wfexec

import (
	"fmt"
	"maps"
	"strings"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/toolhost"
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

	var parentReg toolhost.Registry = rtx.ToolRegistry
	if parentReg == nil {
		r := tools.NewRegistry(rtx.EffectiveWorkspacePath())
		if err := tools.RegisterBuiltinsForConfig(r, rtx.Cfg); err != nil {
			return err
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
		Manifest:        rtx.Manifest,
		UserDataRoot:    rtx.EffectiveUserDataRoot(),
		InstructionRoot: rtx.EffectiveInstructionRoot(),
		SessionRoot:     rtx.EffectiveSessionRoot(),
		ParentWorkspace: rtx.EffectiveWorkspacePath(),
		ProfileID:       rtx.EffectiveProfileID(),
		UseMock:         rtx.EffectiveUseMock(),
		Stdout:          rtx.Stdout,
		OnSubAgentChunk: rtx.OnSubAgentAssistantChunk,
		CorrelationID:   corr,
		ParentRegistry:  parentReg,
		DelegationDepth: rtx.DelegationDepth,
	}
	userPrompt, err := BuildSubagentUserPrompt(rtx)
	if err != nil {
		return err
	}
	_, err = subagent.ExecuteSubAgentTurn(rtx.GoCtx, deps, sub, userPrompt)
	return err
}
