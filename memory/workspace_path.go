package memory

import (
	"path/filepath"
	"strings"
)

// IMWorkspaceDirName is the default subdirectory under InstructionRoot for Engine.CWD (file tools).
const IMWorkspaceDirName = "workspace"

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
	return JoinSessionWorkspaceWithInstruction(cwd, "", flat, elem...)
}

// JoinSessionWorkspaceWithInstruction anchors session runtime files (tasks, exec_log, …).
// When flat and instructionRoot is non-empty, files live directly under <instructionRoot>/… (no nested ".oneclaw"; see docs/user-root-workspace-layout.md).
// Otherwise behavior matches JoinSessionWorkspace (cwd is the flat session root).
func JoinSessionWorkspaceWithInstruction(cwd, instructionRoot string, flat bool, elem ...string) string {
	base := filepath.Clean(cwd)
	if flat && strings.TrimSpace(instructionRoot) != "" {
		base = filepath.Clean(instructionRoot)
	}
	rel := SessionWorkspaceRel(flat, elem...)
	if rel == "" {
		return base
	}
	return filepath.Join(base, rel)
}
