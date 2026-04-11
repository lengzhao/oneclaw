package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveUnderRoot returns the absolute path for userPath resolved under root.
// userPath may be absolute or relative; the result must stay within root.
func ResolveUnderRoot(root, userPath string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return "", err
	}
	var target string
	if filepath.IsAbs(userPath) {
		target = filepath.Clean(userPath)
	} else {
		target = filepath.Clean(filepath.Join(rootAbs, userPath))
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, targetAbs)
	if err != nil {
		return "", fmt.Errorf("path outside working directory")
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes working directory")
	}
	return targetAbs, nil
}
