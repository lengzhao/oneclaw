// Command maintain distills today's daily log → project MEMORY.md.
// By default it runs on a repeating interval (see ONCLAW_MAINTAIN_INTERVAL); use -once for a single pass (e.g. system crontab).
// Uses ONCLAW_MAINTENANCE_SCHEDULED_MODEL → ONCLAW_MAINTENANCE_MODEL → ONCLAW_MODEL / default chat model.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	_ "github.com/lengzhao/conf/autoload"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

func main() {
	cwd := flag.String("cwd", ".", "project working directory (memory layout root)")
	configPath := flag.String("config", "", "path to extra YAML layer (merged after user and project config)")
	logLevel := flag.String("log-level", "", "debug|info|warn|error (overrides ONCLAW_LOG_LEVEL)")
	logFormat := flag.String("log-format", "", "text|json (overrides ONCLAW_LOG_FORMAT)")
	once := flag.Bool("once", false, "run a single distill pass and exit (overrides interval)")
	var intervalExplicit bool
	var intervalFlag time.Duration
	flag.Func("interval", "sleep between passes when looping (default: ONCLAW_MAINTAIN_INTERVAL or 1h; 0 = run once)", func(s string) error {
		intervalExplicit = true
		d, err := time.ParseDuration(s)
		if err != nil {
			return err
		}
		intervalFlag = d
		return nil
	})
	flag.Parse()

	absCwd, err := filepath.Abs(*cwd)
	if err != nil {
		slog.Error("cwd abs", "err", err)
		os.Exit(1)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Error("user home", "err", err)
		os.Exit(1)
	}

	cfg, err := config.Load(config.LoadOptions{Cwd: absCwd, Home: home, ExplicitPath: *configPath})
	if err != nil {
		slog.Error("config.load", "err", err)
		os.Exit(1)
	}
	config.ApplyEnvDefaults(cfg)
	logx.Init(cfg.LogLevel(*logLevel), cfg.LogFormat(*logFormat))

	if !cfg.HasAPIKey() {
		slog.Error("missing API key: set openai.api_key in config or OPENAI_API_KEY")
		os.Exit(1)
	}

	layout := memory.DefaultLayout(absCwd, home)
	client := openai.NewClient(cfg.OpenAIOptions()...)
	mainModel := string(openai.ChatModelGPT4o)
	if m := cfg.ChatModel(); m != "" {
		mainModel = m
	}

	const maxMaintainOut = int64(8192)
	loopInterval := cfg.MaintainLoopInterval()
	if intervalExplicit {
		loopInterval = intervalFlag
	}
	if *once {
		loopInterval = 0
	}
	if loopInterval > 0 && memory.ScheduledMaintenanceBackgroundDisabled() {
		slog.Info("memory.maintain.scheduled_skip", "reason", "background_disabled", "hint", "ONCLAW_DISABLE_SCHEDULED_MAINTENANCE or features.disable_scheduled_maintenance")
		return
	}
	reg := builtin.ScheduledMaintainReadRegistry()
	if loopInterval <= 0 {
		slog.Info("memory.maintain.scheduled_pass", "reason", "once", "cwd", absCwd)
		memory.RunScheduledMaintain(context.Background(), layout, &client, mainModel, maxMaintainOut,
			&memory.ScheduledMaintainOpts{ToolRegistry: reg})
		return
	}
	opts := &memory.ScheduledMaintainOpts{Interval: loopInterval, ToolRegistry: reg}
	slog.Info("memory.maintain.scheduler", "mode", "interval", "every", loopInterval.String(), "cwd", absCwd)
	for {
		slog.Info("memory.maintain.scheduled_pass", "reason", "interval", "every", loopInterval.String(), "cwd", absCwd)
		memory.RunScheduledMaintain(context.Background(), layout, &client, mainModel, maxMaintainOut, opts)
		time.Sleep(loopInterval)
	}
}
