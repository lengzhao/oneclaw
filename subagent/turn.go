package subagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/lengzhao/oneclaw/adkhost"
	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/observe"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/session"
)

func newSubRunID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "sub-" + hex.EncodeToString(b[:])
}

// ExecuteSubAgentTurn runs a sub-agent with a fresh message list and optional subs layout (phase 4).
func ExecuteSubAgentTurn(ctx context.Context, deps *RunAgentDeps, sub *catalog.Agent, userContent string) (string, error) {
	if deps == nil || sub == nil {
		return "", fmt.Errorf("subagent: ExecuteSubAgentTurn: nil deps or agent")
	}
	if deps.ParentRegistry == nil {
		return "", fmt.Errorf("subagent: ParentRegistry required")
	}
	if deps.Catalog == nil || deps.Cfg == nil {
		return "", fmt.Errorf("subagent: Catalog and Cfg required")
	}

	maxD := adkhost.MaxDelegationDepth(deps.Cfg)
	if deps.DelegationDepth >= maxD {
		return "", fmt.Errorf("subagent: max delegation depth %d reached", maxD)
	}

	subRunID := newSubRunID()
	parentSessionRoot := deps.SessionRoot
	subSessionRoot := paths.SubSessionRoot(parentSessionRoot, subRunID)
	if err := os.MkdirAll(subSessionRoot, 0o755); err != nil {
		return "", err
	}

	mode := strings.ToLower(strings.TrimSpace(sub.Workspace))
	if mode == "" {
		mode = "shared"
	}
	var childWS string
	switch mode {
	case "shared":
		childWS = deps.ParentWorkspace
	case "private":
		childWS = paths.Workspace(subSessionRoot)
		if err := os.MkdirAll(childWS, 0o755); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("invalid workspace mode %q (want shared or private)", sub.Workspace)
	}

	memOpts := &preturn.BuildOpts{OmitMemory: !sub.InheritParentMemory}
	bundle, err := preturn.Build(deps.UserDataRoot, deps.InstructionRoot, sub, preturn.DefaultBudget(), memOpts)
	if err != nil {
		return "", err
	}

	runTmpl := &RunAgentDeps{
		Turn: TurnBinding{
			SessionSegment:  deps.Turn.SessionSegment,
			InboundClientID: deps.Turn.InboundClientID,
			AgentID:         sub.AgentType,
		},
		Catalog:         deps.Catalog,
		Cfg:             deps.Cfg,
		UserDataRoot:    deps.UserDataRoot,
		InstructionRoot: deps.InstructionRoot,
		SessionRoot:     deps.SessionRoot,
		ParentWorkspace: childWS,
		ProfileID:       deps.ProfileID,
		UseMock:         deps.UseMock,
		Stdout:          deps.Stdout,
		OnSubAgentChunk: deps.OnSubAgentChunk,
		CorrelationID:   deps.CorrelationID,
		DelegationDepth: deps.DelegationDepth + 1,
		ParentRegistry:  deps.ParentRegistry,
	}
	childReg, err := BuildRegistryForAgent(childWS, bundle.ToolAllowlist, deps.ParentRegistry, runTmpl)
	if err != nil {
		return "", err
	}

	profID := strings.TrimSpace(sub.Model)
	if profID == "" {
		profID = strings.TrimSpace(deps.ProfileID)
	}
	prof, err := config.ResolveModelProfile(deps.Cfg, profID)
	if err != nil {
		return "", fmt.Errorf("sub-agent %q: %w", sub.AgentType, err)
	}
	useMock := deps.UseMock || strings.EqualFold(prof.Provider, "mock")
	cm, err := adkhost.NewToolCallingChatModel(ctx, prof, useMock)
	if err != nil {
		return "", fmt.Errorf("sub-agent %q: %w", sub.AgentType, err)
	}

	desc := sub.Description
	if desc == "" {
		desc = sub.Name
	}
	maxIt := adkhost.MaxAgentIterations(deps.Cfg)

	runCtx := observe.WithAgentRunAttrs(ctx, observe.AgentRunAttrs{
		CorrelationID:   deps.CorrelationID,
		ParentSessionID: deps.Turn.SessionSegment,
		SubRunID:        subRunID,
	})

	agentRun, err := adkhost.NewChatModelAgent(runCtx, cm, childReg, adkhost.AgentOptions{
		Name:          sub.AgentType,
		Description:   desc,
		Instruction:   bundle.Instruction,
		MaxIterations: maxIt,
		Handlers:      []adk.ChatModelAgentMiddleware{observe.NewChatModelLogMiddleware()},
	})
	if err != nil {
		return "", err
	}

	corrDetail := map[string]any{
		"correlation_id":    deps.CorrelationID,
		"parent_session_id": deps.Turn.SessionSegment,
		"sub_run_id":        subRunID,
		"workspace_mode":    mode,
	}
	now := time.Now().UTC()
	if err := session.AppendRunEvent(subSessionRoot, sub.AgentType, session.RunEvent{
		Ts: now, AgentType: sub.AgentType, Phase: "sub_agent_start",
		Detail: corrDetail,
	}); err != nil {
		return "", err
	}
	if err := session.AppendTranscriptTurn(subSessionRoot, session.TranscriptTurn{
		Ts: now, Role: "user", Content: strings.TrimSpace(userContent),
	}); err != nil {
		return "", err
	}

	input := &adk.AgentInput{
		Messages: []adk.Message{schema.UserMessage(strings.TrimSpace(userContent))},
	}
	var chunks []string
	iter := agentRun.Run(runCtx, input)
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			return "", ev.Err
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil && ev.Output.MessageOutput.Message != nil {
			msg := ev.Output.MessageOutput.Message
			c := strings.TrimSpace(msg.Content)
			if c == "" {
				continue
			}
			chunks = append(chunks, c)
			if deps.OnSubAgentChunk != nil {
				deps.OnSubAgentChunk(deps.CorrelationID, subRunID, sub.AgentType, c)
			}
			if deps.Stdout != nil {
				_, _ = fmt.Fprintln(deps.Stdout, c)
			}
		}
	}
	reply := strings.TrimSpace(strings.Join(chunks, "\n"))

	end := time.Now().UTC()
	if reply != "" {
		if err := session.AppendTranscriptTurn(subSessionRoot, session.TranscriptTurn{
			Ts: end, Role: "assistant", Content: reply,
		}); err != nil {
			return "", err
		}
	}
	endDetail := map[string]any{
		"correlation_id":    deps.CorrelationID,
		"parent_session_id": deps.Turn.SessionSegment,
		"sub_run_id":        subRunID,
		"reply_len":         len(reply),
	}
	if err := session.AppendRunEvent(subSessionRoot, sub.AgentType, session.RunEvent{
		Ts: end, AgentType: sub.AgentType, Phase: "sub_agent_complete",
		Detail: endDetail,
	}); err != nil {
		return "", err
	}

	return reply, nil
}
