package workspace

import (
	"fmt"
	"path/filepath"
	"strings"
)

// MaxWorkspaceWriteBytes caps single write/append payloads (PicoClaw-style safety rail).
const MaxWorkspaceWriteBytes = 16 << 20

// ResolveUnderWorkspace maps a workspace-relative path to an absolute path under workspaceRoot.
// Empty relPath is treated as "." (workspace root). Rejects ".." segments and paths that escape root.
func ResolveUnderWorkspace(workspaceRoot, relPath string) (abs string, err error) {
	base := filepath.Clean(strings.TrimSpace(workspaceRoot))
	if base == "" {
		return "", fmt.Errorf("workspace: root required")
	}
	rel := filepath.ToSlash(strings.TrimSpace(relPath))
	if rel == "" {
		rel = "."
	}
	if strings.Contains(rel, "..") {
		return "", fmt.Errorf("invalid path")
	}
	localRel := filepath.FromSlash(rel)
	if localRel != "." && filepath.IsAbs(localRel) {
		return "", fmt.Errorf("absolute paths not allowed")
	}
	full := filepath.Clean(filepath.Join(base, localRel))
	out, err := filepath.Rel(base, full)
	if err != nil || strings.HasPrefix(out, "..") {
		return "", fmt.Errorf("path escapes workspace")
	}
	return full, nil
}
