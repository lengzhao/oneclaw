package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/wfexec"
	"github.com/lengzhao/oneclaw/workflow"
)

func runInteractive(ctx context.Context, g globalOpts, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	buf := &strings.Builder{}
	fs.SetOutput(buf)
	mockLLM := fs.Bool("mock-llm", false, "use stub ChatModel (no external API)")
	profileID := fs.String("profile", "", "model profile id (see config models[]; default: highest priority)")
	agentID := fs.String("agent", "", "catalog agent id: *.md filename stem (default: manifest default_agent)")
	prompt := fs.String("prompt", "Say hello in one short sentence.", "single-turn user message")
	sessionID := fs.String("session", "cli-default", "session id for layout under UserDataRoot (unsafe chars replaced)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("run: %w\n%s", err, buf.String())
	}

	cfgPaths := []string{}
	if cp := strings.TrimSpace(g.ConfigPath); cp != "" {
		cfgPaths = append(cfgPaths, cp)
	} else {
		// Without -config, still load ~/.oneclaw/config.yaml (or ONECLAW_USER_DATA_ROOT/config.yaml) when present.
		root, err := paths.ResolveUserDataRoot(nil)
		if err != nil {
			return fmt.Errorf("resolve default user data root: %w", err)
		}
		candidate := filepath.Join(root, "config.yaml")
		if _, err := os.Stat(candidate); err == nil {
			cfgPaths = append(cfgPaths, candidate)
		}
	}
	cfg, err := config.LoadMerged(cfgPaths)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config.ApplyEnvSecrets(cfg)
	config.PushRuntime(cfg)

	root, err := paths.ResolveUserDataRoot(cfg)
	if err != nil {
		return err
	}

	catRoot := paths.CatalogRoot(root)
	mf, err := catalog.LoadManifest(catRoot)
	if err != nil {
		return err
	}
	cat, err := catalog.Load(filepath.Join(catRoot, "agents"))
	if err != nil {
		return err
	}

	at := strings.TrimSpace(*agentID)
	if at == "" {
		at = mf.DefaultAgent
	}
	ag := cat.Get(at)
	if ag == nil {
		return fmt.Errorf("unknown agent id %q (run init; check agents/)", at)
	}

	sessSeg := paths.SanitizeSessionPathSegment(*sessionID)
	sessionRoot := paths.SessionRoot(root, sessSeg)
	instruction := paths.InstructionRoot(root, sessSeg, cfg.IsolateInstructionOrDefault())
	ws := paths.Workspace(instruction)

	for _, d := range []string{ws, sessionRoot} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	if err := paths.SeedInstructionFiles(root, instruction); err != nil {
		return err
	}

	bundle, err := preturn.Build(root, instruction, ag, preturn.DefaultBudget())
	if err != nil {
		return err
	}

	baseReg := tools.NewRegistry(ws)
	if err := tools.RegisterBuiltins(baseReg); err != nil {
		return err
	}
	chosen := baseReg.All()
	if len(bundle.ToolAllowlist) > 0 {
		chosen, err = baseReg.FilterByNames(bundle.ToolAllowlist)
		if err != nil {
			return err
		}
	}
	execReg := tools.NewRegistry(ws)
	for _, t := range chosen {
		if err := execReg.Register(t); err != nil {
			return err
		}
	}

	prof, err := config.ResolveModelProfile(cfg, *profileID)
	if err != nil {
		return err
	}

	var cm model.ToolCallingChatModel
	useMock := *mockLLM || strings.EqualFold(prof.Provider, "mock")
	if useMock {
		cm = adkhost.NewStubChatModel("Hello from oneclaw stub model.")
		slog.InfoContext(ctx, "using stub ChatModel", "profile", prof.ID, "provider", prof.Provider)
	} else {
		openaiCM, err := adkhost.NewOpenAIChatModel(ctx, prof)
		if err != nil {
			return fmt.Errorf("%w (use --mock-llm for offline)", err)
		}
		cm = openaiCM
	}

	desc := ag.Description
	if desc == "" {
		desc = ag.Name
	}
	agent, err := adkhost.NewChatModelAgent(ctx, cm, execReg, adkhost.AgentOptions{
		Name:          ag.AgentType,
		Description:   desc,
		Instruction:   bundle.Instruction,
		MaxIterations: cfg.Runtime.MaxAgentIterations,
		Handlers:      []adk.ChatModelAgentMiddleware{observe.NewChatModelLogMiddleware()},
	})
	if err != nil {
		return err
	}

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
	if err := session.AppendTranscriptTurn(sessionRoot, session.TranscriptTurn{
		Ts: now, Role: "user", Content: *prompt,
	}); err != nil {
		return err
	}
	if err := session.AppendRunEvent(sessionRoot, ag.AgentType, session.RunEvent{
		Ts: now, AgentType: ag.AgentType, Phase: "run_start",
		Detail: map[string]any{
			"mock_llm": useMock, "profile": prof.ID, "model": prof.DefaultModel,
			"session_id": sessSeg, "workflow": wfDoc.ID, "workflow_file": wfPath,
		},
	}); err != nil {
		return err
	}

	reg := wfexec.NewRegistry()
	if err := wfexec.RegisterPhase3Builtins(reg); err != nil {
		return err
	}
	rtx := &engine.RuntimeContext{
		Turn:            engine.TurnContext{AgentID: ag.AgentType},
		SessionRoot:     sessionRoot,
		SessionSegment:  sessSeg,
		Agent:           ag,
		Bundle:          bundle,
		UserPrompt:      *prompt,
		Catalog:         cat,
		Cfg:             cfg,
		UserDataRoot:    root,
		InstructionRoot: instruction,
		WorkspacePath:   ws,
		ChatAgent:       agent,
		Stdout:          os.Stdout,
		RunStartedAt:    now,
		UseMock:         useMock,
		ProfileID:       prof.ID,
		ModelName:       prof.DefaultModel,
	}
	if err := wfexec.Execute(ctx, wfDoc, reg, rtx); err != nil {
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
		Detail: map[string]any{"assistant_len": len(rtx.Assistant), "session_id": sessSeg, "workflow": wfDoc.ID},
	})
}
