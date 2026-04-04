// Package logx configures the default slog logger for oneclaw.
package logx

import (
	"log/slog"
	"os"
	"strings"
)

// Init configures slog from environment and optional CLI overrides.
// Env: ONCLAW_LOG_LEVEL (debug|info|warn|error), ONCLAW_LOG_FORMAT (text|json).
// Empty overrides keep env-only behavior.
func Init(levelOverride, formatOverride string) {
	level := parseLevel(firstNonEmpty(levelOverride, os.Getenv("ONCLAW_LOG_LEVEL")))
	format := strings.ToLower(strings.TrimSpace(firstNonEmpty(formatOverride, os.Getenv("ONCLAW_LOG_FORMAT"))))
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

func firstNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
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
