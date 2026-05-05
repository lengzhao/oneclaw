package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lengzhao/oneclaw/setup"
)

func cmdInit(ctx context.Context, g globalOpts, args []string) error {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	buf := &strings.Builder{}
	fs.SetOutput(buf)
	userData := fs.String("user-data", "", "UserDataRoot directory (default: ~/.oneclaw or ONECLAW_USER_DATA_ROOT)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("init: %w\n%s", err, buf.String())
	}

	root, err := ResolveUserDataRootForCLI(g, *userData)
	if err != nil {
		return err
	}

	if err := setup.Bootstrap(root); err != nil {
		return err
	}
	slog.InfoContext(ctx, "init complete", "user_data_root", root)
	fmt.Fprintf(os.Stdout, "Initialized oneclaw layout under %s\n", root)
	return nil
}
