package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/keyfiles"
	"github.com/lengzhao/oneclaw/setup"
)

func cmdOnboard(ctx context.Context, g globalOpts, args []string) error {
	fs := flag.NewFlagSet("onboard", flag.ContinueOnError)
	buf := &strings.Builder{}
	fs.SetOutput(buf)
	userData := fs.String("user-data", "", "UserDataRoot directory (default: ~/.oneclaw or ONECLAW_USER_DATA_ROOT)")
	mockLLM := fs.Bool("mock-llm", false, "skip LLM API key step and model profile merge (dev/CI)")
	skipLLM := fs.Bool("skip-llm", false, "skip LLM onboarding and keep default/existing model config")
	skipDrivers := fs.Bool("skip-drivers", false, "skip channel drivers onboarding and keep existing clawbridge clients")
	listen := fs.String("listen", "", "when onboarding driver weixin: http_listen for browser QR page (default: terminal QR only)")
	noBrowser := fs.Bool("no-browser", false, "do not open browser (console URLs printed only)")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("onboard: %w\n%s", err, buf.String())
	}

	root, err := ResolveUserDataRootForCLI(g, *userData)
	if err != nil {
		return err
	}
	if err := setup.Bootstrap(root); err != nil {
		return err
	}
	if err := keyfiles.Ensure(root); err != nil {
		return err
	}

	runChannels := func() error {
		if *skipDrivers {
			fmt.Fprintln(os.Stdout, "Skipping channel drivers onboarding (--skip-drivers).")
			return nil
		}
		for {
			driver, skip, err := setup.RunChannelDriverPick(os.Stdout, os.Stdin)
			if err != nil {
				return err
			}
			if skip || driver == "" {
				return nil
			}
			wargs := []string{driver}
			if driver == "weixin" {
				if ls := strings.TrimSpace(*listen); ls != "" {
					wargs = append(wargs, "-listen", ls)
				}
			}
			if err := cmdChannelOnboard(ctx, g, wargs); err != nil {
				return err
			}
			more, err := setup.RunChannelContinuePrompt(os.Stdout, os.Stdin)
			if err != nil {
				return err
			}
			if !more {
				return nil
			}
		}
	}

	runLLMAndPersist := func() error {
		if *skipLLM {
			fmt.Fprintln(os.Stdout, "Skipping LLM onboarding (--skip-llm).")
			return nil
		}
		if *mockLLM {
			fmt.Fprintln(os.Stdout, "Skipping LLM API key setup (--mock-llm).")
			return nil
		}
		prof, defaultModel, skipped, err := setup.RunLLMProviderOnboard(os.Stdout, os.Stdin, root, *noBrowser)
		if err != nil {
			return err
		}
		if skipped {
			return nil
		}
		return persistOnboardModelProfile(g, root, prof, defaultModel)
	}

	if err := runLLMAndPersist(); err != nil {
		return err
	}
	if err := runChannels(); err != nil {
		return err
	}

	enabled, err := hasEnabledClient(g, root)
	if err != nil {
		return err
	}
	if enabled {
		fmt.Fprintf(os.Stdout, "\nOnboarding finished. Start the bridge with: oneclaw serve\n")
	} else {
		fmt.Fprintln(os.Stdout, "\nOnboarding finished, but no enabled channel client found.")
		fmt.Fprintln(os.Stdout, "Next step: run `oneclaw channel onboard <driver>` or set `clawbridge.clients[].enabled: true`.")
	}
	slog.InfoContext(ctx, "onboard complete", "user_data_root", root)
	return nil
}

func persistOnboardModelProfile(g globalOpts, rootGuess string, prof config.ModelProfile, defaultModel string) error {
	cfgPaths := loadConfigPathCandidates(g, rootGuess)
	f, err := config.LoadMerged(cfgPaths)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	cfgPath, err := resolvedConfigWritePath(g, f)
	if err != nil {
		return err
	}

	config.UpsertModelProfile(f, prof)
	f.DefaultModel = defaultModel
	config.ApplyDefaults(f)
	if err := config.Validate(f); err != nil {
		return err
	}
	if err := config.Save(cfgPath, f); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	slog.Info("merged model profile from onboard", "path", cfgPath, "default_model", defaultModel)
	return nil
}

func hasEnabledClient(g globalOpts, rootGuess string) (bool, error) {
	cfgPaths := loadConfigPathCandidates(g, rootGuess)
	f, err := config.LoadMerged(cfgPaths)
	if err != nil {
		return false, fmt.Errorf("load config: %w", err)
	}
	for _, c := range f.Clawbridge.Clients {
		if c.Enabled {
			return true, nil
		}
	}
	return false, nil
}
