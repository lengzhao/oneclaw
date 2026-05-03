package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
)

func cmdRun(ctx context.Context, g globalOpts, args []string) error {
	return runInteractive(ctx, g, args)
}

func cmdSnapshot(ctx context.Context, g globalOpts, args []string) error {
	slog.InfoContext(ctx, "snapshot: not implemented yet", "config", g.ConfigPath, "args", args)
	return nil
}

func cmdVersion(_ context.Context) error {
	_, err := fmt.Fprintf(os.Stdout, "%s\n", version)
	return err
}
