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
	upgradeWorkflows := fs.Bool("upgrade-workflows", false, "After bootstrap, overwrite workflows/*.yaml from embedded templates (fixes stale skill_generator/memory_extractor/default.turn)")
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
	if *upgradeWorkflows {
		if err := setup.SyncWorkflowTemplatesFromEmbed(root); err != nil {
			return fmt.Errorf("init --upgrade-workflows: %w", err)
		}
		slog.InfoContext(ctx, "init upgraded workflows from embedded templates", "user_data_root", root)
		fmt.Fprintf(os.Stdout, "Synced embedded workflows → %s/workflows\n", root)
	}
	slog.InfoContext(ctx, "init complete", "user_data_root", root)
	fmt.Fprintf(os.Stdout, "Initialized oneclaw layout under %s\n", root)
	return nil
}
