package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/routing/cli"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

func main() {
	configPath := flag.String("config", "", "path to extra YAML layer (merged after ~/.oneclaw/config.yaml and <cwd>/.oneclaw/config.yaml)")
	flag.Parse()

	absCwd, err := os.Getwd()
	if err != nil {
		slog.Error("getwd", "err", err)
		os.Exit(1)
	}
	absCwd, err = filepath.Abs(absCwd)
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
	logx.Init(cfg.LogLevel(""), cfg.LogFormat(""))

	eng := session.NewEngine(absCwd, builtin.DefaultRegistry())
	eng.Client = openai.NewClient(cfg.OpenAIOptions()...)
	if m := cfg.ChatModel(); m != "" {
		eng.Model = m
	}
	eng.ChatTransport = cfg.ChatTransport()
	eng.SinkRegistry = routing.DefaultRegistry()

	eng.TranscriptPath = cfg.TranscriptPath()
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

	if !cfg.HasAPIKey() {
		slog.Error("missing API key: set openai.api_key in config or OPENAI_API_KEY",
			"user_config", filepath.Join(home, config.UserRelPath),
			"project_config", filepath.Join(absCwd, memory.DotDir, "config.yaml"),
		)
		os.Exit(1)
	}

	if err := cli.RunREPL(cli.REPLConfig{Engine: eng}); err != nil {
		slog.Error("cli.repl", "err", err)
		os.Exit(1)
	}
}
