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

	sessWire := strings.TrimSpace(*sessionID)

	useMock := *mockLLM
	prof, err := config.ResolveModelProfile(cfg, strings.TrimSpace(*profileID))
	if err != nil {
		return err
	}
	useMock = useMock || strings.EqualFold(prof.Provider, "mock")

	return runner.ExecuteTurn(runner.Params{
		Ctx:            ctx,
		UserDataRoot:   root,
		Config:         cfg,
		Catalog:        cat,
		Manifest:       mf,
		AgentID:        strings.TrimSpace(*agentID),
		ProfileID:      strings.TrimSpace(*profileID),
		SessionSegment: sessWire,
		UserPrompt:     *prompt,
		UseMock:        useMock,
		Stdout:         os.Stdout,
		CorrelationID:  subagent.NewCorrelationID(),
	})
}
