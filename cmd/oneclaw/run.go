package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/runner"
	"github.com/lengzhao/oneclaw/subagent"
)

func runInteractive(ctx context.Context, g globalOpts, args []string) error {
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	buf := &strings.Builder{}
	fs.SetOutput(buf)
	mockLLM := fs.Bool("mock-llm", false, "use stub ChatModel (no external API)")
	profileID := fs.String("profile", "", "profile_id_or_provider/model (see config; overrides agent frontmatter model)")
	agentID := fs.String("agent", "", "catalog agent id: *.md filename stem (default: config catalog.default_agent)")
	prompt := fs.String("prompt", "Say hello in one short sentence.", "single-turn user message")
	sessionID := fs.String("session", "cli-default", "session id for layout under UserDataRoot (unsafe chars replaced)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("run: %w\n%s", err, buf.String())
	}

	cfgPaths, err := mergedConfigPaths(g)
	if err != nil {
		return fmt.Errorf("resolve config paths: %w", err)
	}
	cfg, err := config.LoadMerged(cfgPaths)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	root, err := paths.ResolveUserDataRoot(cfg)
	if err != nil {
		return err
	}
	config.ApplyUserDataSecrets(root, cfg)
	config.PushRuntime(cfg)

	catRoot := paths.CatalogRoot(root)
	cat, err := catalog.Load(filepath.Join(catRoot, "agents"))
	if err != nil {
		return err
	}

	sessWire := strings.TrimSpace(*sessionID)

	at := strings.TrimSpace(*agentID)
	if at == "" {
		at = cfg.ResolvedDefaultAgent()
	}
	ag := cat.Get(at)
	if ag == nil {
		return fmt.Errorf("unknown agent id %q (run init; check agents/)", at)
	}

	useMock := *mockLLM
	sel := config.EffectiveModelSelector(strings.TrimSpace(*profileID), ag.Model)
	prof, err := config.ResolveModelForTurn(cfg, sel)
	if err != nil {
		return err
	}
	useMock = useMock || strings.EqualFold(prof.Provider, "mock")

	return runner.ExecuteTurn(runner.Params{
		Ctx:            ctx,
		UserDataRoot:   root,
		Config:         cfg,
		Catalog:        cat,
		AgentID:        at,
		ProfileID:      strings.TrimSpace(*profileID),
		SessionSegment: sessWire,
		UserPrompt:     *prompt,
		UseMock:        useMock,
		Stdout:         os.Stdout,
		CorrelationID:  subagent.NewCorrelationID(),
	})
}
