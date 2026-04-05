// Command maintain distills today's daily log → project MEMORY.md.
// By default it runs on a repeating interval (see ONCLAW_MAINTAIN_INTERVAL); use -once for a single pass (e.g. cron).
// Uses ONCLAW_MAINTENANCE_SCHEDULED_MODEL → ONCLAW_MAINTENANCE_MODEL → ONCLAW_MODEL / default chat model.
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/lengzhao/conf/autoload"

	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/openai/openai-go"
)

// defaultMaintainLoopInterval returns the sleep between passes when no -once.
// Env ONCLAW_MAINTAIN_INTERVAL: Go duration (e.g. 30m, 1h). Empty → 1h. 0 / off / false → 0 (run once unless -interval overrides).
func defaultMaintainLoopInterval() time.Duration {
	v := strings.TrimSpace(os.Getenv("ONCLAW_MAINTAIN_INTERVAL"))
	if v == "" {
		return time.Hour
	}
	if v == "0" || strings.EqualFold(v, "off") || strings.EqualFold(v, "false") {
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		slog.Warn("maintain.invalid_interval_env", "ONCLAW_MAINTAIN_INTERVAL", v, "fallback", "1h")
		return time.Hour
	}
	return d
}

func main() {
	cwd := flag.String("cwd", ".", "project working directory (memory layout root)")
	logLevel := flag.String("log-level", "", "debug|info|warn|error (overrides ONCLAW_LOG_LEVEL)")
	logFormat := flag.String("log-format", "", "text|json (overrides ONCLAW_LOG_FORMAT)")
	once := flag.Bool("once", false, "run a single distill pass and exit (overrides interval; use for cron)")
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

	logx.Init(*logLevel, *logFormat)

	if os.Getenv("OPENAI_API_KEY") == "" {
		slog.Error("set OPENAI_API_KEY")
		os.Exit(1)
	}

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

	layout := memory.DefaultLayout(absCwd, home)
	client := openai.NewClient()
	mainModel := string(openai.ChatModelGPT4o)
	if m := strings.TrimSpace(os.Getenv("ONCLAW_MODEL")); m != "" {
		mainModel = m
	}

	opt := memory.MaintainOptions{
		MainChatModel:   mainModel,
		MaxOutputTokens: 8192,
		Scheduled:       true,
	}
	loopInterval := defaultMaintainLoopInterval()
	if intervalExplicit {
		loopInterval = intervalFlag
	}
	if *once {
		loopInterval = 0
	}
	if loopInterval <= 0 {
		memory.RunMaintain(context.Background(), layout, &client, opt)
		return
	}
	slog.Info("memory.maintain.scheduler", "interval", loopInterval.String(), "cwd", absCwd)
	for {
		memory.RunMaintain(context.Background(), layout, &client, opt)
		time.Sleep(loopInterval)
	}
}
