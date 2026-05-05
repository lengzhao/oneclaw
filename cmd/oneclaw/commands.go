package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime/debug"
)

func cmdRun(ctx context.Context, g globalOpts, args []string) error {
	return runInteractive(ctx, g, args)
}

func cmdSnapshot(ctx context.Context, g globalOpts, args []string) error {
	slog.InfoContext(ctx, "snapshot: not implemented yet", "config", g.ConfigPath, "args", args)
	return nil
}

func cmdVersion(_ context.Context) error {
	if _, err := fmt.Fprintf(os.Stdout, "%s\n", version); err != nil {
		return err
	}
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Path != "" {
		_, _ = fmt.Fprintf(os.Stdout, "module %s\n", bi.Main.Path)
	}
	return nil
}
