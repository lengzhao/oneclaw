package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/lengzhao/conf/autoload"

	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/routing/cli"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

func main() {
	logx.Init("", "")

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

	eng := session.NewEngine(absCwd, builtin.DefaultRegistry())
	eng.SinkRegistry = routing.DefaultRegistry()

	transcriptPath := resolveTranscriptPath(absCwd)
	if transcriptPath != "" {
		if b, err := os.ReadFile(transcriptPath); err == nil {
			if err := eng.LoadTranscript(b); err != nil {
				slog.Error("load transcript", "err", err)
				os.Exit(1)
			}
		} else if !os.IsNotExist(err) {
			slog.Error("read transcript", "path", transcriptPath, "err", err)
			os.Exit(1)
		}
	}
	eng.TranscriptPath = transcriptPath

	if os.Getenv("OPENAI_API_KEY") == "" {
		slog.Error("set OPENAI_API_KEY")
		os.Exit(1)
	}

	if err := cli.RunREPL(cli.REPLConfig{Engine: eng}); err != nil {
		slog.Error("cli.repl", "err", err)
		os.Exit(1)
	}
}

// resolveTranscriptPath picks the default session transcript file under cwd unless disabled or overridden by env.
func resolveTranscriptPath(cwd string) string {
	if transcriptDisabled() {
		return ""
	}
	p := strings.TrimSpace(os.Getenv("ONCLAW_TRANSCRIPT_PATH"))
	if p == "" {
		return filepath.Join(cwd, ".oneclaw", "transcript.json")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(filepath.Join(cwd, p))
	if err != nil {
		return filepath.Join(cwd, p)
	}
	return abs
}

func transcriptDisabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_TRANSCRIPT"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}
