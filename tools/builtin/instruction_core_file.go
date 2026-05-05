package builtin

import (
	"fmt"
	"path/filepath"
	"strings"
)

var allowedInstructionFiles = map[string]string{
	"agent.md":  "AGENT.md",
	"memory.md": "MEMORY.md",
	"soul.md":   "SOUL.md",
	"user.md":   "USER.md",
}

func resolveInstructionCoreFile(root, rel string) (string, error) {
	r := strings.TrimSpace(rel)
	if r == "" {
		return "", fmt.Errorf("path required")
	}
	key := strings.ToLower(filepath.ToSlash(r))
	if v, ok := allowedInstructionFiles[key]; ok {
		return filepath.Join(root, v), nil
	}
	return "", fmt.Errorf("path must be one of AGENT.md, MEMORY.md, SOUL.md, USER.md")
}

func resolveInstructionCoreFileMaybe(root, rel string) (full string, matched bool, err error) {
	key := strings.ToLower(filepath.ToSlash(strings.TrimSpace(rel)))
	_, ok := allowedInstructionFiles[key]
	if !ok {
		return "", false, nil
	}
	if strings.TrimSpace(root) == "" {
		return "", true, fmt.Errorf("instruction root required for %s", rel)
	}
	full, err = resolveInstructionCoreFile(root, rel)
	return full, true, err
}
