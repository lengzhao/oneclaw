package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveUnderAllowedRoots resolves userPath to an absolute path that must stay under cwd or under one of extraRoots.
func ResolveUnderAllowedRoots(cwd string, extraRoots []string, userPath string) (string, error) {
	cwdAbs, err := filepath.Abs(filepath.Clean(cwd))
	if err != nil {
		return "", err
	}
	var target string
	if filepath.IsAbs(userPath) {
		target = filepath.Clean(userPath)
	} else {
		target = filepath.Clean(filepath.Join(cwdAbs, userPath))
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if isUnderRoot(targetAbs, cwdAbs) {
		return targetAbs, nil
	}
	for _, root := range extraRoots {
		rootAbs, err := filepath.Abs(filepath.Clean(root))
		if err != nil {
			continue
		}
		if isUnderRoot(targetAbs, rootAbs) {
			return targetAbs, nil
		}
	}
	return "", fmt.Errorf("path outside working directory and memory roots")
}

func isUnderRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
