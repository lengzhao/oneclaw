package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/lengzhao/clawbridge"
	"github.com/lengzhao/clawbridge/bus"
	_ "github.com/lengzhao/clawbridge/drivers"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/maintainloop"
	"github.com/lengzhao/oneclaw/mcpclient"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

func main() {
	configPath := flag.String("config", "", "path to extra YAML layer (merged after ~/.oneclaw/config.yaml and <cwd>/.oneclaw/config.yaml)")
	cwdFlag := flag.String("cwd", "", "project root directory (default: current working directory)")
	maintainOnce := flag.Bool("maintain-once", false, "run one scheduled memory distill pass and exit (no channels)")
	initFlag := flag.Bool("init", false, "create <cwd>/.oneclaw; write config.yaml from built-in example if missing, else merge in missing keys without overwriting user values; then exit")
	logLevel := flag.String("log-level", "", "debug|info|warn|error (overrides config log.level when non-empty)")
	logFormat := flag.String("log-format", "", "text|json (overrides config log.format when non-empty)")
	flag.Parse()

	if *maintainOnce && *initFlag {
		slog.Error("cli.usage", "err", "use only one of -maintain-once or -init")
		os.Exit(2)
	}

	absCwd, err := resolveCwd(*cwdFlag)
	if err != nil {
		slog.Error("cwd", "err", err)
		os.Exit(1)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("user home", "err", err)
		os.Exit(1)
	}

	if *initFlag {
		logx.Init(*logLevel, *logFormat)
		if err := config.InitWorkspace(absCwd, home); err != nil {
			slog.Error("init", "err", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Load(config.LoadOptions{Cwd: absCwd, Home: home, ExplicitPath: *configPath})
	if err != nil {
		slog.Error("config.load", "err", err)
		os.Exit(1)
	}
	cfg.PushRuntime()
	logx.Init(cfg.LogLevel(*logLevel), cfg.LogFormat(*logFormat))

	if *maintainOnce {
		if !cfg.HasAPIKey() {
			slog.Error("missing API key: set openai.api_key in config",
				"user_config", filepath.Join(home, config.UserRelPath),
				"project_config", filepath.Join(absCwd, memory.DotDir, "config.yaml"),
			)
			os.Exit(1)
		}
		client := openai.NewClient(cfg.OpenAIOptions()...)
		mainModel := string(openai.ChatModelGPT4o)
		if m := cfg.ChatModel(); m != "" {
			mainModel = m
		}
		maxTok := memory.MaintenanceMaxOutputTokens(8192)
		reg := builtin.ScheduledMaintainReadRegistry()
		slog.Info("memory.maintain.scheduled_pass", "reason", "maintain-once", "cwd", absCwd)
		memory.RunScheduledMaintain(context.Background(), memory.DefaultLayout(absCwd, home), &client, mainModel, maxTok,
			&memory.ScheduledMaintainOpts{ToolRegistry: reg})
		return
	}

	eng := session.NewEngine(absCwd, builtin.DefaultRegistry())
	eng.Client = openai.NewClient(cfg.OpenAIOptions()...)
	if m := cfg.ChatModel(); m != "" {
		eng.Model = m
	}
	eng.ChatTransport = cfg.ChatTransport()

	eng.TranscriptPath = cfg.TranscriptPath()
	eng.WorkingTranscriptPath = cfg.WorkingTranscriptPath()
	eng.WorkingTranscriptMaxMessages = cfg.WorkingTranscriptMaxMessages()
	if eng.TranscriptPath != "" {
		if b, err := os.ReadFile(eng.TranscriptPath); err == nil {
			if err := eng.LoadTranscript(b); err != nil {
				slog.Error("load transcript", "err", err)
				os.Exit(1)
			}
		} else if !os.IsNotExist(err) {
			slog.Error("read transcript", "path", eng.TranscriptPath, "err", err)
			os.Exit(1)
		}
	}
	if eng.WorkingTranscriptPath != "" {
		if b, err := os.ReadFile(eng.WorkingTranscriptPath); err == nil {
			if err := eng.LoadWorkingTranscript(b); err != nil {
				slog.Error("load working transcript", "err", err)
				os.Exit(1)
			}
		} else if !os.IsNotExist(err) {
			slog.Error("read working transcript", "path", eng.WorkingTranscriptPath, "err", err)
			os.Exit(1)
		}
	}

	if !cfg.HasAPIKey() {
		slog.Error("missing API key: set openai.api_key in config",
			"user_config", filepath.Join(home, config.UserRelPath),
			"project_config", filepath.Join(absCwd, memory.DotDir, "config.yaml"),
		)
		os.Exit(1)
	}

	rootCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpMgr, mcpNote, err := mcpclient.RegisterIfEnabled(rootCtx, cfg, eng.Registry, absCwd)
	if err != nil {
		slog.Error("mcp.register", "err", err)
		os.Exit(1)
	}
	if mcpMgr != nil {
		defer func() { _ = mcpMgr.Close() }()
	}
	if mcpNote != "" {
		eng.MCPSystemNote = mcpNote
	}

	maintainloop.Start(rootCtx, maintainloop.Params{
		Interval:          cfg.EmbeddedScheduledMaintainInterval(),
		Layout:            memory.DefaultLayout(absCwd, home),
		Client:            &eng.Client,
		MainModel:         eng.Model,
		MaxMaintainTokens: eng.MaxTokens,
	})

	cbCfg, err := cfg.ClawbridgeConfigForRun()
	if err != nil {
		slog.Error("clawbridge.config", "err", err)
		os.Exit(1)
	}
	if len(cbCfg.Clients) == 0 {
		slog.Error("no IM clients in config",
			"hint", "add clawbridge.clients (driver feishu, slack, noop, webchat, …) under clawbridge: in config; see https://github.com/lengzhao/clawbridge/blob/main/config.example.yaml",
			"cwd", absCwd,
		)
		os.Exit(1)
	}

	bridge, err := clawbridge.New(cbCfg)
	if err != nil {
		slog.Error("clawbridge.new", "err", err)
		os.Exit(1)
	}
	eng.PublishOutbound = func(ctx context.Context, msg *bus.OutboundMessage) error {
		return bridge.Bus().PublishOutbound(ctx, msg)
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
		if err := schedule.StartHostPollerIfEnabled(rootCtx, absCwd, c.ID, eng.SubmitUser); err != nil {
			slog.Error("schedule.poller", "client_id", c.ID, "err", err)
			os.Exit(1)
		}
	}

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
			go func() {
				submitCtx := context.Background()
				if err := eng.SubmitUser(submitCtx, m); err != nil {
					slog.Warn("session.submit_user", "err", err)
				}
			}()
		}
	}()

	<-rootCtx.Done()
}

func resolveCwd(flagCwd string) (string, error) {
	if flagCwd != "" {
		return filepath.Abs(flagCwd)
	}
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Abs(wd)
}
