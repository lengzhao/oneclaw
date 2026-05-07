package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/paths"
)

// mergedConfigPaths returns YAML paths using the same rules as run/serve:
// explicit -config, else <default UserDataRoot>/config.yaml when that file exists.
func mergedConfigPaths(g globalOpts) ([]string, error) {
	if cp := strings.TrimSpace(g.ConfigPath); cp != "" {
		return []string{cp}, nil
	}
	rootGuess, err := paths.ResolveUserDataRoot(nil)
	if err != nil {
		return nil, err
	}
	candidate := filepath.Join(rootGuess, "config.yaml")
	if _, err := os.Stat(candidate); err == nil {
		return []string{candidate}, nil
	}
	return nil, nil
}
