package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/lengzhao/oneclaw/workspace"
)

// InitWorkspace creates <home>/.oneclaw when needed (first argument is the user home directory or parent of the dot dir). It copies `config/init_template/` into the directory
// (config.yaml, AGENT.md, memory/MEMORY.md, MAINTAIN_SCHEDULED.md, MAINTAIN_POST_TURN.md, …) for any file that does not already exist (never overwrites user files). If `config.yaml` already existed
// before this copy, merges in any keys from the embedded template that the file lacks (recursive for maps);
// existing user values are never overwritten. Arrays are kept as-is when the key exists. If the merge adds
// keys, the file is rewritten (YAML comments may be lost).
func InitWorkspace(cwd, home string) error {
	if cwd == "" {
		return fmt.Errorf("config.init: empty cwd")
	}
	dot := filepath.Join(cwd, workspace.DotDir)
	if err := os.MkdirAll(dot, 0o755); err != nil {
		return fmt.Errorf("config.init: mkdir %s: %w", dot, err)
	}
	cfgPath := filepath.Join(dot, "config.yaml")
	configExisted := false
	if _, err := os.Stat(cfgPath); err == nil {
		configExisted = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("config.init: stat %s: %w", cfgPath, err)
	}

	tmplBytes, err := initTemplateConfigYAML()
	if err != nil {
		return fmt.Errorf("config.init: read embedded template config: %w", err)
	}
	if len(tmplBytes) == 0 {
		return fmt.Errorf("config.init: embedded template config is empty")
	}

	if err := copyInitTemplate(dot); err != nil {
		return err
	}

	if configExisted {
		userBytes, err := os.ReadFile(cfgPath)
		if err != nil {
			return fmt.Errorf("config.init: read %s: %w", cfgPath, err)
		}
		merged, changed, err := mergeInitYAML(tmplBytes, userBytes)
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
	} else {
		slog.Info("config.init.template", "path", cfgPath, "reason", "copied_from_init_template")
	}

	workspace.DefaultLayout(cwd, home).EnsureDirs()
	slog.Info("config.init.dirs", "cwd", cwd)
	return nil
}
