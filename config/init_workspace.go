package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/memory"
)

// InitWorkspace creates <cwd>/.oneclaw when needed. If config.yaml is missing, writes the embedded
// example. If it already exists, merges in any keys from the embedded example that the file lacks
// (recursive for mappings); existing user values are never overwritten. Arrays are kept as-is when
// the key exists. If the merge adds keys, the file is rewritten (YAML comments may be lost).
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
		userBytes, err := os.ReadFile(cfgPath)
		if err != nil {
			return fmt.Errorf("config.init: read %s: %w", cfgPath, err)
		}
		if len(projectInitExampleYAML) == 0 {
			return fmt.Errorf("config.init: embedded example is empty")
		}
		merged, changed, err := mergeInitYAML(projectInitExampleYAML, userBytes)
		if err != nil {
			return err
		}
		if changed {
			if err := os.WriteFile(cfgPath, merged, 0o644); err != nil {
				return fmt.Errorf("config.init: write %s: %w", cfgPath, err)
			}
			slog.Info("config.init.merged", "path", cfgPath, "reason", "filled_missing_keys_from_template")
		} else {
			slog.Info("config.init.skip_config", "path", cfgPath, "reason", "no_missing_keys")
		}
	}
	memory.DefaultLayout(cwd, home).EnsureDirs()
	slog.Info("config.init.dirs", "cwd", cwd)
	return nil
}
