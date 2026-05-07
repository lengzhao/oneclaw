package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/paths"
	"gopkg.in/yaml.v3"
)

func cmdConfig(_ context.Context, g globalOpts, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: config show [flags]\nTry: oneclaw config show -h")
	}
	switch args[0] {
	case "show":
		return cmdConfigShow(g, args[1:])
	default:
		return fmt.Errorf("config: unknown subcommand %q (try show)", args[0])
	}
}

func cmdConfigShow(g globalOpts, args []string) error {
	fs := flag.NewFlagSet("config show", flag.ContinueOnError)
	buf := &strings.Builder{}
	fs.SetOutput(buf)
	rawOnly := fs.Bool("raw", false, "show merged YAML only — do not apply env vars or key_files")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("config show: %w\n%s", err, buf.String())
	}

	cfgPaths, err := mergedConfigPaths(g)
	if err != nil {
		return err
	}
	cfg, err := config.LoadMerged(cfgPaths)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	root, err := paths.ResolveUserDataRoot(cfg)
	if err != nil {
		return err
	}
	if !*rawOnly {
		config.ApplyUserDataSecrets(root, cfg)
	}

	red, err := config.RedactSecretsDeepCopy(cfg)
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "# merged_config_paths: %v\n", cfgPaths)
	fmt.Fprintf(os.Stdout, "# user_data_root: %s\n", root)
	if *rawOnly {
		fmt.Fprintf(os.Stdout, "# effective_secrets: not_applied (--raw)\n")
	} else {
		fmt.Fprintf(os.Stdout, "# effective_secrets: applied (env + key_files), values redacted below\n")
	}

	out, err := yaml.Marshal(red)
	if err != nil {
		return err
	}
	if _, err := os.Stdout.Write(out); err != nil {
		return err
	}
	return nil
}
