package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/observe"
)

// version is set by -ldflags at release; default "dev".
var version = "dev"

func main() {
	os.Exit(run(os.Args))
}

func run(argv []string) int {
	prog := "oneclaw"
	if len(argv) > 0 {
		prog = filepath.Base(argv[0])
	}

	args := argv[1:]
	if len(args) == 0 {
		writeUsage(os.Stderr, prog)
		return 2
	}

	if args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		writeUsage(os.Stdout, prog)
		return 0
	}
	if args[0] == "-v" || args[0] == "--version" || args[0] == "version" {
		ctx := context.Background()
		if err := cmdVersion(ctx); err != nil {
			slog.Error("version", "err", err)
			return 1
		}
		return 0
	}

	g, rest, err := parseLeadingFlags(args)
	if err != nil {
		slog.Error("flags", "err", err)
		writeUsage(os.Stderr, prog)
		return 2
	}
	if err := observeSetup(g); err != nil {
		slog.Error("observe", "err", err)
		return 1
	}

	if len(rest) == 0 {
		writeUsage(os.Stderr, prog)
		return 2
	}

	ctx := context.Background()
	cmd := rest[0]
	cmdArgs := rest[1:]

	switch cmd {
	case "help":
		writeUsage(os.Stdout, prog)
		return 0
	case "version":
		if err := cmdVersion(ctx); err != nil {
			slog.Error("version", "err", err)
			return 1
		}
		return 0
	case "init":
		if err := cmdInit(ctx, g, cmdArgs); err != nil {
			slog.Error("init", "err", err)
			return 1
		}
		return 0
	case "run", "repl":
		if err := cmdRun(ctx, g, cmdArgs); err != nil {
			slog.Error(cmd, "err", err)
			return 1
		}
		return 0
	case "snapshot":
		if err := cmdSnapshot(ctx, g, cmdArgs); err != nil {
			slog.Error("snapshot", "err", err)
			return 1
		}
		return 0
	case "serve":
		if err := cmdServe(ctx, g, cmdArgs); err != nil {
			slog.Error("serve", "err", err)
			return 1
		}
		return 0
	case "channel":
		if err := cmdChannel(ctx, g, cmdArgs); err != nil {
			slog.Error("channel", "err", err)
			return 1
		}
		return 0
	default:
		slog.Error("unknown command", "cmd", cmd)
		writeUsage(os.Stderr, prog)
		return 2
	}
}

func observeSetup(g globalOpts) error {
	return observe.Setup(observe.LogOptions{
		Level:  g.LogLevel,
		Format: g.LogFormat,
	})
}
