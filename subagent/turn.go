package subagent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"

	"github.com/lengzhao/oneclaw/adkhost"
	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/observe"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/workflow"
)

// subRunAgentSegmentMaxRunes caps the agent_type portion of subs/<id>/ (filesystem-friendly segment length).
const subRunAgentSegmentMaxRunes = 64

func newSubRunID(agentType string) string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	suffix := hex.EncodeToString(b[:])
	at := strings.TrimSpace(agentType)
	if at == "" {
		return "sub-" + suffix
	}
	seg := paths.SanitizeSessionPathSegment(at)
	seg = truncateRunes(seg, subRunAgentSegmentMaxRunes)
	return "sub-" + seg + "-" + suffix
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return s
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

func effectiveParentModelSelector(deps *RunAgentDeps) string {
	if deps == nil {
		return ""
	}
	profile := strings.TrimSpace(deps.ProfileID)
	if _, _, ok := config.SplitProviderModel(profile); ok {
		return profile
	}
	modelName := strings.TrimSpace(deps.ModelName)
	if profile == "" || modelName == "" {
		return profile
	}
	return profile + "/" + modelName
}

// ExecuteSubAgentTurn runs a sub-agent: ResolveWorkflowPath(agent_type) → wfexec.Execute (registered via RegisterWorkflowExecutor).
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

	subRunID := newSubRunID(sub.AgentType)
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
		return "", fmt.Errorf("invalid workspace mode %q (want shared or private)", mode)
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
			ReplyMeta:       maps.Clone(deps.Turn.ReplyMeta),
		},
		HostAgentID:     deps.HostAgentID,
		Catalog:         deps.Catalog,
		Cfg:             deps.Cfg,
		UserDataRoot:    deps.UserDataRoot,
		InstructionRoot: deps.InstructionRoot,
		SessionRoot:     deps.SessionRoot,
		ParentWorkspace: childWS,
		ProfileID:       deps.ProfileID,
		ModelName:       deps.ModelName,
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

	sel := config.EffectiveModelSelector(effectiveParentModelSelector(deps), sub.Model)
	profCandidates, err := config.ResolveModelProfilesForTurn(deps.Cfg, sel)
	if err != nil {
		return "", fmt.Errorf("sub-agent %q: %w", sub.AgentType, err)
	}
	var prof config.ModelProfile
	var cm model.ToolCallingChatModel
	var lastConstructErr error
	for i := range profCandidates {
		pp := profCandidates[i]
		um := deps.UseMock || strings.EqualFold(pp.Provider, "mock")
		c, err := adkhost.NewToolCallingChatModel(ctx, &pp, um)
		if err != nil {
			lastConstructErr = err
			slog.WarnContext(ctx, "subagent: model profile unavailable, trying failover",
				"agent_type", sub.AgentType, "profile_id", pp.ID, "provider", pp.Provider, "err", err)
			continue
		}
		prof = pp
		cm = c
		break
	}
	if cm == nil {
		if lastConstructErr != nil {
			return "", fmt.Errorf("sub-agent %q: no usable model profile after failover: %w", sub.AgentType, lastConstructErr)
		}
		return "", fmt.Errorf("sub-agent %q: no usable model profile after failover", sub.AgentType)
	}
	useMock := deps.UseMock || strings.EqualFold(prof.Provider, "mock")

	desc := sub.Description
	if desc == "" {
		desc = sub.Name
	}
	maxIt := adkhost.MaxAgentIterationsOrCatalog(deps.Cfg, sub.MaxTurns)

	initialInstr := strings.TrimSpace(bundle.Instruction)
	if initialInstr == "" {
		initialInstr = " "
	}

	runCtx := observe.WithAgentRunAttrs(ctx, observe.AgentRunAttrs{
		CorrelationID:   deps.CorrelationID,
		ParentSessionID: deps.Turn.SessionSegment,
		SubRunID:        subRunID,
	})

	agentRun, err := adkhost.NewChatModelAgent(runCtx, cm, childReg, adkhost.AgentOptions{
		Name:          sub.AgentType,
		Description:   desc,
		Instruction:   initialInstr,
		MaxIterations: maxIt,
		Handlers:      []adk.ChatModelAgentMiddleware{observe.NewChatModelLogMiddleware()},
	})
	if err != nil {
		return "", err
	}

	catRoot := paths.CatalogRoot(deps.UserDataRoot)
	wfPath, err := workflow.ResolveWorkflowPath(catRoot, sub.AgentType, deps.Cfg)
	if err != nil {
		return "", fmt.Errorf("sub-agent %q: %w", sub.AgentType, err)
	}
	wfRaw, err := os.ReadFile(wfPath)
	if err != nil {
		return "", fmt.Errorf("sub-agent %q read workflow %s: %w", sub.AgentType, wfPath, err)
	}
	wfDoc, err := workflow.ParseBytes(wfRaw)
	if err != nil {
		return "", fmt.Errorf("sub-agent %q parse workflow %s: %w", sub.AgentType, wfPath, err)
	}
	if err := workflow.Validate(wfDoc); err != nil {
		return "", fmt.Errorf("sub-agent %q workflow %s: %w", sub.AgentType, wfPath, err)
	}

	corrDetail := map[string]any{
		"correlation_id":    deps.CorrelationID,
		"parent_session_id": deps.Turn.SessionSegment,
		"sub_run_id":        subRunID,
		"workspace_mode":    mode,
		"workflow":          wfDoc.ID,
		"workflow_file":     wfPath,
	}
	journalKey := strings.TrimSpace(deps.CorrelationID) + "__" + strings.TrimSpace(subRunID)
	now := time.Now().UTC()
	if err := session.AppendTurnRunEvent(subSessionRoot, sub.AgentType, journalKey, session.RunEvent{
		Ts: now, AgentType: sub.AgentType, Phase: "sub_agent_start",
		Detail: corrDetail,
	}); err != nil {
		return "", err
	}

	streamReply := workflow.ReplyStreamEnabled(wfDoc)
	var onAssistantChunk func(string)
	if streamReply && deps.OnSubAgentChunk != nil {
		onAssistantChunk = func(chunk string) {
			chunk = strings.TrimSpace(chunk)
			if chunk == "" {
				return
			}
			deps.OnSubAgentChunk(deps.CorrelationID, subRunID, sub.AgentType, chunk)
		}
	}

	var stdoutFile *os.File
	if deps.Stdout != nil {
		if f, ok := deps.Stdout.(*os.File); ok {
			stdoutFile = f
		}
	}

	childRTX := engine.ForkSubAgentRuntime(engine.SubAgentRuntimeOpts{
		Turn: engine.TurnContext{
			AgentID:   sub.AgentType,
			ReplyMeta: maps.Clone(deps.Turn.ReplyMeta),
		},
		DelegationDepth: deps.DelegationDepth + 1,
		SubSessionRoot:  subSessionRoot,
		SessionSegment:  deps.Turn.SessionSegment,
		Agent:           sub,
		Bundle:          bundle,
		UserPrompt:      strings.TrimSpace(userContent),
		Catalog:         deps.Catalog,
		Cfg:             deps.Cfg,
		UserDataRoot:    deps.UserDataRoot,
		InstructionRoot: deps.InstructionRoot,
		WorkspacePath:   childWS,
		ToolRegistry:    childReg,
		ChatAgent:       agentRun,
		ChatModel:       cm,
		AgentShellMeta: engine.AgentShellMeta{
			Name:          sub.AgentType,
			Description:   desc,
			MaxIterations: maxIt,
			Handlers:      []adk.ChatModelAgentMiddleware{observe.NewChatModelLogMiddleware()},
		},
		Stdout:          stdoutFile,
		RunStartedAt:    now,
		UseMock:         useMock,
		ProfileID:       prof.ID,
		ModelName:       prof.DefaultModel,
		CorrelationID:   deps.CorrelationID,
		OnSubAgentChunk: deps.OnSubAgentChunk,
	})
	if onAssistantChunk != nil {
		childRTX.OnAssistantChunk = onAssistantChunk
	}

	forceInfo := os.Getenv("ONECLAW_VERBOSE_PROMPT") == "1"
	if forceInfo || slog.Default().Enabled(runCtx, slog.LevelDebug) {
		logFn := slog.DebugContext
		if forceInfo {
			logFn = slog.InfoContext
		}
		logFn(runCtx, "subagent.system_prompt",
			"agent_type", sub.AgentType,
			"sub_run_id", subRunID,
			"chars", len(bundle.Instruction),
			"text", bundle.Instruction,
		)
		u := strings.TrimSpace(userContent)
		logFn(runCtx, "subagent.user_message",
			"agent_type", sub.AgentType,
			"sub_run_id", subRunID,
			"chars", len(u),
			"text", u,
		)
		logFn(runCtx, "subagent.workflow",
			"agent_type", sub.AgentType,
			"workflow_id", wfDoc.ID,
			"path", wfPath,
		)
	}

	if err := runWorkflow(runCtx, wfDoc, childRTX); err != nil {
		return "", err
	}

	reply := strings.TrimSpace(childRTX.Assistant)

	end := time.Now().UTC()
	if !childRTX.SawOnRespond && reply != "" {
		if err := session.AppendTranscriptTurn(subSessionRoot, sub.AgentType, session.TranscriptTurn{
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
		"workflow":          wfDoc.ID,
	}
	if err := session.AppendTurnRunEvent(subSessionRoot, sub.AgentType, journalKey, session.RunEvent{
		Ts: end, AgentType: sub.AgentType, Phase: "sub_agent_complete",
		Detail: endDetail,
	}); err != nil {
		return "", err
	}

	return reply, nil
}
