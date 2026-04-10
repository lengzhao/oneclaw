// Package logx configures the default slog logger for oneclaw.
package logx

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Init sets the default slog logger. When logFile is non-empty, logs are appended to that file (UTF-8)
// in addition to stderr. Returns close, which should be deferred on shutdown to flush and release the file.
func Init(levelOverride, formatOverride, logFile string) (close func()) {
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

	out := io.Writer(os.Stderr)
	var f *os.File
	if p := strings.TrimSpace(logFile); p != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "logx: mkdir for log file %q: %v (stderr only)\n", p, err)
		} else {
			var err error
			f, err = os.OpenFile(p, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				fmt.Fprintf(os.Stderr, "logx: open log file %q: %v (stderr only)\n", p, err)
			} else {
				out = io.MultiWriter(os.Stderr, f)
			}
		}
	}

	var h slog.Handler
	switch format {
	case "json":
		h = slog.NewJSONHandler(out, opts)
	default:
		h = slog.NewTextHandler(out, opts)
	}

	root := slog.New(h).With(
		slog.String("svc", "oneclaw"),
	)
	slog.SetDefault(root)

	if f == nil {
		return func() {}
	}
	return func() { _ = f.Close() }
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
