// Package logx configures the default slog logger for oneclaw.
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// Init configures slog level/format strings (e.g. from config merged with CLI flags).
func Init(levelOverride, formatOverride string) {
	level := parseLevel(levelOverride)
	format := strings.ToLower(strings.TrimSpace(formatOverride))
	if format == "" {
		format = "text"
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}
	if level == slog.LevelDebug {
		opts.AddSource = true
	}

	var h slog.Handler
	switch format {
	case "json":
		h = slog.NewJSONHandler(os.Stderr, opts)
	default:
		h = slog.NewTextHandler(os.Stderr, opts)
	}

	root := slog.New(h).With(
		slog.String("svc", "oneclaw"),
	)
	slog.SetDefault(root)
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
