package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
	"strings"
)

func cmdRun(ctx context.Context, g globalOpts, args []string) error {
	return runInteractive(ctx, g, args)
}

func cmdSnapshot(ctx context.Context, g globalOpts, args []string) error {
	slog.InfoContext(ctx, "snapshot: not implemented yet", "config", g.ConfigPath, "args", args)
	return nil
}

func cmdVersion(_ context.Context) error {
	if _, err := fmt.Fprintf(os.Stdout, "version=%s\n", strings.TrimSpace(version)); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(os.Stdout, "build_marker=%s\n", strings.TrimSpace(buildMarker))
	_, _ = fmt.Fprintf(os.Stdout, "build_commit=%s\n", strings.TrimSpace(buildCommit))
	_, _ = fmt.Fprintf(os.Stdout, "build_time=%s\n", strings.TrimSpace(buildTime))

	if bi, ok := debug.ReadBuildInfo(); ok {
		if bi.Main.Path != "" {
			_, _ = fmt.Fprintf(os.Stdout, "module=%s\n", bi.Main.Path)
		}
		if bi.Main.Version != "" {
			_, _ = fmt.Fprintf(os.Stdout, "module_version=%s\n", bi.Main.Version)
		}
		for _, s := range bi.Settings {
			switch s.Key {
			case "vcs.revision":
				_, _ = fmt.Fprintf(os.Stdout, "vcs_revision=%s\n", s.Value)
			case "vcs.time":
				_, _ = fmt.Fprintf(os.Stdout, "vcs_time=%s\n", s.Value)
			case "vcs.modified":
				_, _ = fmt.Fprintf(os.Stdout, "vcs_modified=%s\n", s.Value)
			}
		}
	}
	return nil
}
