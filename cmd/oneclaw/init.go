package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/paths"
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

	root := strings.TrimSpace(*userData)
	if root == "" && strings.TrimSpace(g.ConfigPath) != "" {
		root = filepath.Dir(strings.TrimSpace(g.ConfigPath))
	}
	if root == "" {
		if v := strings.TrimSpace(os.Getenv(paths.EnvUserDataRoot)); v != "" {
			var err error
			root, err = paths.ExpandHome(v)
			if err != nil {
				return err
			}
		}
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		root = filepath.Join(home, ".oneclaw")
	} else {
		var err error
		root, err = paths.ExpandHome(root)
		if err != nil {
			return err
		}
	}

	if err := setup.Bootstrap(root); err != nil {
		return err
	}
	slog.InfoContext(ctx, "init complete", "user_data_root", root)
	fmt.Fprintf(os.Stdout, "Initialized oneclaw layout under %s\n", root)
	return nil
}
