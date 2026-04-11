package memory

import "path/filepath"

// SessionWorkspaceRel is the relative path under Engine.CWD for session-scoped files.
// flat=true: CWD is already the logical .oneclaw directory (tasks.json, memory/, agents/ live directly under CWD).
// flat=false: legacy layout; files live under CWD/.oneclaw/.
func SessionWorkspaceRel(flat bool, elem ...string) string {
	if flat {
		if len(elem) == 0 {
			return ""
		}
		return filepath.Join(elem...)
	}
	return filepath.Join(append([]string{DotDir}, elem...)...)
}

// JoinSessionWorkspace is filepath.Join(cwd, SessionWorkspaceRel(flat, elem...)).
func JoinSessionWorkspace(cwd string, flat bool, elem ...string) string {
	rel := SessionWorkspaceRel(flat, elem...)
	if rel == "" {
		return filepath.Clean(cwd)
	}
	return filepath.Join(cwd, rel)
}
