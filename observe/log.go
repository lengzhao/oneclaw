// Package observe provides logging setup aligned with FR-CFG-04 / FR-OBS (structured logs).
package observe

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
)

// LogOptions configures the default slog logger (stderr).
type LogOptions struct {
	Level  string // debug, info, warn, error; empty means info
	Format string // text (default) or json
	Output io.Writer
}

// Setup replaces [slog.Default] with a handler derived from opts.
func Setup(opts LogOptions) error {
	lvl, err := ParseLevel(opts.Level)
	if err != nil {
		return err
	}
	out := opts.Output
	if out == nil {
		out = os.Stderr
	}
	format := strings.ToLower(strings.TrimSpace(opts.Format))
	if format == "" {
		format = "text"
	}
	handlerOpts := &slog.HandlerOptions{Level: lvl}
	var h slog.Handler
	switch format {
	case "text":
		h = slog.NewTextHandler(out, handlerOpts)
	case "json":
		h = slog.NewJSONHandler(out, handlerOpts)
	default:
		return fmt.Errorf("observe: unknown log format %q (want text or json)", opts.Format)
	}
	slog.SetDefault(slog.New(h))
	return nil
}

// ParseLevel maps common names to [slog.Level].
func ParseLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("observe: unknown log level %q", s)
	}
}
