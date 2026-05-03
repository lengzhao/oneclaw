package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"

	"github.com/lengzhao/oneclaw/adkhost"
	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/observe"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/wfexec"
	"github.com/lengzhao/oneclaw/workflow"
)

// Params carries one user turn for ExecuteTurn (CLI, HTTP, TurnHub processor, etc.).
type Params struct {
	Ctx context.Context

	UserDataRoot string
	Config       *config.File
	Catalog      *catalog.Catalog
	Manifest     *catalog.Manifest

	AgentID        string // catalog agent id; empty uses manifest default_agent (see InboundMeta* for clawbridge Metadata)
	ProfileID      string // empty uses config default profile resolution
	SessionSegment string // sanitized session path segment (paths.SanitizeSessionPathSegment)
	UserPrompt     string

	UseMock bool
	Stdout  *os.File

	CorrelationID string

	// InboundClientID is set for multi-channel serve turns so scheduled jobs reply on the same client.
	InboundClientID string

	PostAssistantRespond func(context.Context, string) error
}

// ExecuteTurn runs catalog → preturn → workflow → ADK for one inbound user message.
func ExecuteTurn(p Params) error {
	ctx := p.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if p.Config == nil {
		return fmt.Errorf("runner: nil config")
	}
	if p.Catalog == nil {
		return fmt.Errorf("runner: nil catalog")
	}
	mf := p.Manifest
	if mf == nil {
		mf = &catalog.Manifest{DefaultAgent: "default"}
	}

	root := strings.TrimSpace(p.UserDataRoot)
	if root == "" {
		return fmt.Errorf("runner: empty user data root")
	}

	at := strings.TrimSpace(p.AgentID)
	if at == "" {
		at = mf.DefaultAgent
	}
	ag := p.Catalog.Get(at)
	if ag == nil {
		return fmt.Errorf("unknown agent id %q (run init; check agents/)", at)
	}

	sessSeg := paths.SanitizeSessionPathSegment(p.SessionSegment)
	sessionRoot := paths.SessionRoot(root, sessSeg)
	instruction := paths.InstructionRoot(root, sessSeg, p.Config.IsolateInstructionOrDefault())
	ws := paths.Workspace(instruction)

	for _, d := range []string{ws, sessionRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if err := paths.SeedInstructionFiles(root, instruction); err != nil {
		return err
	}

	bundle, err := preturn.Build(root, instruction, ag, preturn.DefaultBudget(), nil)
	if err != nil {
		return err
	}

	prof, err := config.ResolveModelProfile(p.Config, strings.TrimSpace(p.ProfileID))
	if err != nil {
		return err
	}

	useMock := p.UseMock || strings.EqualFold(prof.Provider, "mock")
	corrID := strings.TrimSpace(p.CorrelationID)
	if corrID == "" {
		corrID = subagent.NewCorrelationID()
	}
	deps := &subagent.RunAgentDeps{
		Turn: subagent.TurnBinding{
			SessionSegment:  sessSeg,
			InboundClientID: strings.TrimSpace(p.InboundClientID),
			AgentID:         at,
		},
		Catalog:         p.Catalog,
		Cfg:             p.Config,
		UserDataRoot:    root,
		InstructionRoot: instruction,
		SessionRoot:     sessionRoot,
		ParentWorkspace: ws,
		ProfileID:       prof.ID,
		UseMock:         useMock,
		Stdout:          p.Stdout,
		CorrelationID:   corrID,
	}
	execReg, err := subagent.BuildExecRegistry(ws, bundle.ToolAllowlist, deps)
	if err != nil {
		return err
	}

	if useMock {
		slog.InfoContext(ctx, "using stub ChatModel", "profile", prof.ID, "provider", prof.Provider)
	}

	cm, err := adkhost.NewToolCallingChatModel(ctx, prof, useMock)
	if err != nil {
		return fmt.Errorf("%w (use --mock-llm for offline)", err)
	}

	desc := ag.Description
	if desc == "" {
		desc = ag.Name
	}
	agent, err := adkhost.NewChatModelAgent(ctx, cm, execReg, adkhost.AgentOptions{
		Name:          ag.AgentType,
		Description:   desc,
		Instruction:   bundle.Instruction,
		MaxIterations: adkhost.MaxAgentIterations(p.Config),
		Handlers:      []adk.ChatModelAgentMiddleware{observe.NewChatModelLogMiddleware()},
	})
	if err != nil {
		return err
	}

	catRoot := paths.CatalogRoot(root)
	wfPath, err := wfexec.ResolveWorkflowPath(catRoot, ag.AgentType, mf)
	if err != nil {
		return err
	}
	wfRaw, err := os.ReadFile(wfPath)
	if err != nil {
		return fmt.Errorf("read workflow %s: %w", wfPath, err)
	}
	wfDoc, err := workflow.ParseBytes(wfRaw)
	if err != nil {
		return fmt.Errorf("parse workflow %s: %w", wfPath, err)
	}
	if err := workflow.Validate(wfDoc); err != nil {
		return fmt.Errorf("workflow %s: %w", wfPath, err)
	}

	now := time.Now().UTC()
	prompt := strings.TrimSpace(p.UserPrompt)
	if err := session.AppendTranscriptTurn(sessionRoot, session.TranscriptTurn{
		Ts: now, Role: "user", Content: prompt,
	}); err != nil {
		return err
	}
	if err := session.AppendRunEvent(sessionRoot, ag.AgentType, session.RunEvent{
		Ts: now, AgentType: ag.AgentType, Phase: "run_start",
		Detail: map[string]any{
			"mock_llm": useMock, "profile": prof.ID, "model": prof.DefaultModel,
			"session_id": sessSeg, "correlation_id": corrID, "workflow": wfDoc.ID, "workflow_file": wfPath,
		},
	}); err != nil {
		return err
	}

	rtx := &engine.RuntimeContext{
		Turn:                 engine.TurnContext{AgentID: ag.AgentType},
		SessionRoot:          sessionRoot,
		SessionSegment:       sessSeg,
		Agent:                ag,
		Bundle:               bundle,
		UserPrompt:           prompt,
		Catalog:              p.Catalog,
		Cfg:                  p.Config,
		UserDataRoot:         root,
		InstructionRoot:      instruction,
		WorkspacePath:        ws,
		ToolRegistry:         execReg,
		ChatAgent:            agent,
		Stdout:               p.Stdout,
		RunStartedAt:         now,
		UseMock:              useMock,
		ProfileID:            prof.ID,
		ModelName:            prof.DefaultModel,
		CorrelationID:        corrID,
		PostAssistantRespond: p.PostAssistantRespond,
	}
	reg := wfexec.NewRegistry()
	if err := wfexec.RegisterPhase3Builtins(reg); err != nil {
		return err
	}
	if err := wfexec.Execute(ctx, wfDoc, reg, rtx); err != nil {
		slog.ErrorContext(ctx, "workflow execute failed",
			"err", err,
			"workflow", wfDoc.ID,
			"workflow_file", wfPath,
			"session_id", sessSeg,
			"correlation_id", corrID,
			"agent", ag.AgentType,
		)
		return err
	}

	end := time.Now().UTC()
	if !rtx.SawOnRespond && strings.TrimSpace(rtx.Assistant) != "" {
		if err := session.AppendTranscriptTurn(sessionRoot, session.TranscriptTurn{
			Ts: end, Role: "assistant", Content: rtx.Assistant,
		}); err != nil {
			return err
		}
	}
	return session.AppendRunEvent(sessionRoot, ag.AgentType, session.RunEvent{
		Ts: end, AgentType: ag.AgentType, Phase: "run_complete",
		Detail: map[string]any{"assistant_len": len(rtx.Assistant), "session_id": sessSeg, "correlation_id": corrID, "workflow": wfDoc.ID},
	})
}
