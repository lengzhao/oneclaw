package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/memory"
)

// InitWorkspace creates <cwd>/.oneclaw when needed, writes config.yaml from the embedded example
// if that file does not exist, then ensures memory directories and default AGENT.md.
func InitWorkspace(cwd, home string) error {
	if cwd == "" {
		return fmt.Errorf("config.init: empty cwd")
	}
	dot := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		return fmt.Errorf("config.init: mkdir %s: %w", dot, err)
	}
	cfgPath := filepath.Join(dot, "config.yaml")
	if _, err := os.Stat(cfgPath); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("config.init: stat %s: %w", cfgPath, err)
		}
		if len(projectInitExampleYAML) == 0 {
			return fmt.Errorf("config.init: embedded example is empty")
		}
		if err := os.WriteFile(cfgPath, projectInitExampleYAML, 0o644); err != nil {
			return fmt.Errorf("config.init: write %s: %w", cfgPath, err)
		}
		slog.Info("config.init.wrote", "path", cfgPath)
	} else {
		slog.Info("config.init.skip_config", "path", cfgPath, "reason", "already_exists")
	}
	memory.DefaultLayout(cwd, home).EnsureDirs()
	slog.Info("config.init.dirs", "cwd", cwd)
	return nil
}
