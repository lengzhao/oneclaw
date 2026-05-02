package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/lengzhao/clawbridge"
	"github.com/lengzhao/clawbridge/bus"
	_ "github.com/lengzhao/clawbridge/drivers"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/mcpclient"
	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/lengzhao/oneclaw/workspace"
	"github.com/openai/openai-go"
)

func main() {
	configPath := flag.String("config", "", "path to extra YAML layer (merged after ~/.oneclaw/config.yaml; relative paths are under ~/.oneclaw/)")
	initFlag := flag.Bool("init", false, "create ~/.oneclaw from template; merge config keys if config.yaml already exists; if stdin is a TTY, prompt for openai, model, sessions.isolate_workspace, clawbridge.clients preset; then exit")
	exportSession := flag.String("export-session", "", "copy host data from ~/.oneclaw into this directory, then exit (no API key required)")
	logLevel := flag.String("log-level", "", "debug|info|warn|error (overrides config log.level when non-empty)")
	logFormat := flag.String("log-format", "", "text|json (overrides config log.format when non-empty)")
	logFile := flag.String("log-file", "", "append logs to this file (UTF-8) in addition to stderr; overrides config log.file when non-empty; relative to ~/.oneclaw after first load, else ~/.oneclaw for early init/export")
	flag.Parse()

	exclusive := 0
	if *exportSession != "" {
		exclusive++
	}
	if *initFlag {
		exclusive++
	}
	if exclusive > 1 {
		slog.Error("cli.usage", "err", "use only one of -export-session or -init")
		os.Exit(2)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("user home", "err", err)
		os.Exit(1)
	}
	userDataRoot := filepath.Join(home, workspace.DotDir)

	var logClose func()
	defer func() {
		if logClose != nil {
			logClose()
		}
	}()

	if *initFlag {
		logClose = logx.Init(*logLevel, *logFormat, config.ResolveLogPath(userDataRoot, *logFile))
		if err := config.InitWorkspace(home, home); err != nil {
			slog.Error("init", "err", err)
			os.Exit(1)
		}
		cfgPath := filepath.Join(userDataRoot, "config.yaml")
		if err := config.PromptInitIfTerminal(cfgPath, os.Stdin, os.Stdout); err != nil {
			slog.Error("init", "err", err)
			os.Exit(1)
		}
		return
	}

	if *exportSession != "" {
		logClose = logx.Init(*logLevel, *logFormat, config.ResolveLogPath(userDataRoot, *logFile))
		if err := workspace.ExportSessionSnapshot(userDataRoot, *exportSession); err != nil {
			slog.Error("export-session", "err", err)
			os.Exit(1)
		}
		slog.Info("export-session.done", "data_root", userDataRoot, "out", *exportSession)
		return
	}

	cfg, err := config.Load(config.LoadOptions{Home: home, ExplicitPath: *configPath})
	if err != nil {
		slog.Error("config.load", "err", err)
		os.Exit(1)
	}
	cfg.PushRuntime()
	logClose = logx.Init(cfg.LogLevel(*logLevel), cfg.LogFormat(*logFormat), cfg.LogFile(*logFile))

	sharedReg := builtin.DefaultRegistry()
	mainModel := string(openai.ChatModelGPT4o)
	if m := cfg.ChatModel(); m != "" {
		mainModel = m
	}

	if !cfg.HasAPIKey() {
		slog.Error("missing API key: set openai.api_key in config",
			"user_config", filepath.Join(home, config.UserRelPath),
		)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpMgr, mcpNote, err := mcpclient.RegisterIfEnabled(rootCtx, cfg, sharedReg, cfg.UserDataRoot())
	if err != nil {
		slog.Error("mcp.register", "err", err)
		os.Exit(1)
	}
	if mcpMgr != nil {
		defer func() { _ = mcpMgr.Close() }()
	}

	cbCfg, err := cfg.ClawbridgeConfigForRun()
	if err != nil {
		slog.Error("clawbridge.config", "err", err)
		os.Exit(1)
	}
	if len(cbCfg.Clients) == 0 {
		slog.Error("no IM clients in config",
			"hint", "add clawbridge.clients (driver feishu, slack, noop, webchat, …) under clawbridge: in config; see https://github.com/lengzhao/clawbridge/blob/main/config.example.yaml",
			"data_root", cfg.UserDataRoot(),
		)
		os.Exit(1)
	}
	bridge, err := clawbridge.New(cbCfg)
	if err != nil {
		slog.Error("clawbridge.new", "err", err)
		os.Exit(1)
	}
	deps := session.MainEngineFactoryDeps{
		Resolved:      cfg,
		Registry:      sharedReg,
		Model:         mainModel,
		MCPSystemNote: mcpNote,
		Bridge:        bridge,
	}
	engineFactory := session.MainEngineFactory(deps)

	turnPolicy := session.ParseTurnPolicy(cfg.SessionTurnPolicyRaw())
	turnHub, err := session.NewTurnHub(rootCtx, turnPolicy, engineFactory)
	if err != nil {
		slog.Error("session.turn_hub", "err", err)
		os.Exit(1)
	}
	defer turnHub.Close()

	submitInbound := func(ctx context.Context, m bus.InboundMessage) error {
		h := session.SessionHandle{Source: m.ClientID, SessionKey: session.InboundSessionKey(m)}
		if session.IsStopSlashCommand(m.Content) {
			turnHub.CancelInflightTurn(h)
		}
		return turnHub.Submit(ctx, m)
	}

	if err := bridge.Start(rootCtx); err != nil {
		slog.Error("clawbridge.start", "err", err)
		os.Exit(1)
	}
	defer func() {
		stopCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := bridge.Stop(stopCtx); err != nil {
			slog.Warn("clawbridge.stop", "err", err)
		}
	}()

	for _, c := range cbCfg.Clients {
		if !c.Enabled {
			continue
		}
		if err := schedule.StartHostPollerIfEnabled(rootCtx, cfg.UserDataRoot(), cfg.SessionIsolateWorkspace(), c.ID, submitInbound); err != nil {
			slog.Error("schedule.poller", "client_id", c.ID, "err", err)
			os.Exit(1)
		}
	}

	var inboundInflight sync.WaitGroup
	go func() {
		for {
			msg, err := bridge.Bus().ConsumeInbound(rootCtx)
			if err != nil {
				if errors.Is(err, context.Canceled) || errors.Is(err, bus.ErrClosed) {
					return
				}
				slog.Error("clawbridge.consume_inbound", "err", err)
				return
			}
			m := msg
			inboundInflight.Add(1)
			go func() {
				defer inboundInflight.Done()
				if err := submitInbound(rootCtx, m); err != nil {
					slog.Warn("session.submit_user", "err", err)
				}
			}()
		}
	}()

	<-rootCtx.Done()
	inboundInflight.Wait()
}
