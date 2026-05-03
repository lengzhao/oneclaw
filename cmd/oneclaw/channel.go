package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/clawbridge/client"
	cbconfig "github.com/lengzhao/clawbridge/config"
	_ "github.com/lengzhao/clawbridge/drivers"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/paths"
)

func cmdChannel(ctx context.Context, g globalOpts, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("channel: subcommand required: onboard | list-drivers")
	}
	switch args[0] {
	case "onboard":
		return cmdChannelOnboard(ctx, g, args[1:])
	case "list-drivers":
		return cmdChannelListDrivers()
	default:
		return fmt.Errorf("channel: unknown subcommand %q (try onboard or list-drivers)", args[0])
	}
}

func cmdChannelListDrivers() error {
	ds := client.ListOnboardingDrivers()
	if len(ds) == 0 {
		fmt.Fprintln(os.Stdout, "(no drivers registered — import clawbridge drivers)")
		return nil
	}
	for _, d := range ds {
		fmt.Fprintln(os.Stdout, d)
	}
	return nil
}

func cmdChannelOnboard(ctx context.Context, g globalOpts, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: channel onboard <driver> [flags]\nExample: oneclaw channel onboard weixin -listen 127.0.0.1:8769")
	}
	driver := strings.TrimSpace(args[0])
	if driver == "" {
		return fmt.Errorf("channel onboard: empty driver name")
	}
	rest := args[1:]

	fs := flag.NewFlagSet("channel onboard", flag.ContinueOnError)
	buf := &strings.Builder{}
	fs.SetOutput(buf)
	clientID := fs.String("client-id", "", "clawbridge client id (default: <driver>-1)")
	allowFrom := fs.String("allow-from", "*", "comma-separated allow_from list")
	stateDir := fs.String("state-dir", "", "optional state dir (driver-specific)")
	proxy := fs.String("proxy", "", "optional HTTP proxy URL")
	listen := fs.String("listen", "", "weixin: http_listen for browser QR page (e.g. 127.0.0.1:8769)")
	printMode := fs.String("print", "human", "after onboarding: none | human | yaml | json")
	printSecrets := fs.Bool("print-secrets", false, "show secrets in report (default masked)")
	dryRun := fs.Bool("dry-run", false, "run onboarding but do not write config.yaml")
	if err := fs.Parse(rest); err != nil {
		return fmt.Errorf("channel onboard: %w\n%s", err, buf.String())
	}

	cid := strings.TrimSpace(*clientID)
	if cid == "" {
		cid = driver + "-1"
	}

	spec := client.NewOnboarding(driver, cid).
		WithAllowFrom(splitAllowFromCSV(*allowFrom)...).
		WithStateDir(strings.TrimSpace(*stateDir)).
		WithProxy(strings.TrimSpace(*proxy))

	params := map[string]any{}
	if ls := strings.TrimSpace(*listen); ls != "" {
		params["http_listen"] = ls
	}
	if len(params) > 0 {
		spec = spec.WithParams(params)
	}

	pm, err := client.ParseOnboardingPrintMode(strings.TrimSpace(*printMode))
	if err != nil {
		return err
	}

	res, err := client.RunOnboarding(ctx, spec)
	if err != nil {
		return fmt.Errorf("onboarding: %w", err)
	}

	client.ReportOnboarding(os.Stdout, pm, res, client.ReportOptions{
		MaskSecrets: !*printSecrets,
		ErrWriter:   os.Stderr,
	})

	if *dryRun {
		slog.Info("dry-run: not writing config")
		return nil
	}

	if !res.Ready() {
		if res.Phase == client.OnboardingPhaseManual {
			return persistClawbridgeMerge(g, res.Config, "wrote manual clawbridge client stub (set enabled: true after filling options)")
		}
		return fmt.Errorf("onboarding finished without a runnable client (phase=%s)", res.Phase)
	}

	return persistClawbridgeMerge(g, res.Config, "merged clawbridge into config")
}

func persistClawbridgeMerge(g globalOpts, patch cbconfig.Config, msg string) error {
	rootGuess, err := paths.ResolveUserDataRoot(nil)
	if err != nil {
		return err
	}
	cfgPaths := loadConfigPathCandidates(g, rootGuess)
	f, err := config.LoadMerged(cfgPaths)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	config.ApplyEnvSecrets(f)

	cfgPath, err := resolvedConfigWritePath(g, f)
	if err != nil {
		return err
	}

	config.UpsertClawbridge(f, patch)
	if err := config.Save(cfgPath, f); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	slog.Info(msg, "path", cfgPath)
	return nil
}

func loadConfigPathCandidates(g globalOpts, rootGuess string) []string {
	if cp := strings.TrimSpace(g.ConfigPath); cp != "" {
		return []string{cp}
	}
	candidate := filepath.Join(rootGuess, "config.yaml")
	if _, err := os.Stat(candidate); err == nil {
		return []string{candidate}
	}
	return nil
}

func resolvedConfigWritePath(g globalOpts, f *config.File) (string, error) {
	if cp := strings.TrimSpace(g.ConfigPath); cp != "" {
		return cp, nil
	}
	root, err := paths.ResolveUserDataRoot(f)
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "config.yaml"), nil
}

func splitAllowFromCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}
