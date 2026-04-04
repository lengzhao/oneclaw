package main

import (
	"flag"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/logx"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/routing/cli"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

func main() {
	cwd := flag.String("cwd", ".", "working directory for tools")
	transcript := flag.String("transcript", "", "load/save transcript path (optional)")
	logLevel := flag.String("log-level", "", "log level: debug, info, warn, error (overrides ONCLAW_LOG_LEVEL)")
	logFormat := flag.String("log-format", "", "log format: text or json (overrides ONCLAW_LOG_FORMAT)")
	flag.Parse()

	logx.Init(*logLevel, *logFormat)

	absCwd, err := os.Getwd()
	if err != nil {
		slog.Error("getwd", "err", err)
		os.Exit(1)
	}
	if *cwd != "." {
		absCwd = *cwd
	}
	absCwd, err = filepath.Abs(absCwd)
	if err != nil {
		slog.Error("cwd abs", "err", err)
		os.Exit(1)
	}

	eng := session.NewEngine(absCwd, builtin.DefaultRegistry())
	eng.SinkRegistry = routing.DefaultRegistry()
	if *transcript != "" {
		if b, err := os.ReadFile(*transcript); err == nil {
			if err := eng.LoadTranscript(b); err != nil {
				slog.Error("load transcript", "err", err)
				os.Exit(1)
			}
		}
	}

	if os.Getenv("OPENAI_API_KEY") == "" {
		slog.Error("set OPENAI_API_KEY")
		os.Exit(1)
	}

	if err := cli.RunREPL(cli.REPLConfig{
		Engine:         eng,
		TranscriptPath: *transcript,
	}); err != nil {
		slog.Error("cli.repl", "err", err)
		os.Exit(1)
	}
}
