package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/paths"
)

// ResolveUserDataRootForCLI returns UserDataRoot for init/onboard (same rules as init --user-data).
func ResolveUserDataRootForCLI(g globalOpts, userDataFlag string) (string, error) {
	root := strings.TrimSpace(userDataFlag)
	if root == "" && strings.TrimSpace(g.ConfigPath) != "" {
		root = filepath.Dir(strings.TrimSpace(g.ConfigPath))
	}
	if root == "" {
		if v := strings.TrimSpace(os.Getenv(paths.EnvUserDataRoot)); v != "" {
			return paths.ExpandHome(v)
		}
	}
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		root = filepath.Join(home, ".oneclaw")
		return root, nil
	}
	return paths.ExpandHome(root)
}
