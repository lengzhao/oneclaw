package workspace

import (
	"os"
	"path/filepath"
	"strings"
)

// IMWorkspaceDirName is the default subdirectory under InstructionRoot for Engine.CWD (file tools).
const IMWorkspaceDirName = "workspace"

// SessionWorkspaceRel is the relative path under Engine.CWD for session-scoped files.
// Session-scoped files always live directly under the resolved session/runtime root.
func SessionWorkspaceRel(flat bool, elem ...string) string {
	_ = flat
	if len(elem) == 0 {
		return ""
	}
	return filepath.Join(elem...)
}

// JoinSessionWorkspace is filepath.Join(cwd, SessionWorkspaceRel(flat, elem...)).
func JoinSessionWorkspace(cwd string, flat bool, elem ...string) string {
	return JoinSessionWorkspaceWithInstruction(cwd, "", flat, elem...)
}

// JoinSessionWorkspaceWithInstruction anchors session runtime files (tasks, exec_log, …).
// When flat and instructionRoot is non-empty, files live directly under <instructionRoot>/…
// Otherwise behavior matches JoinSessionWorkspace.
func JoinSessionWorkspaceWithInstruction(cwd, instructionRoot string, flat bool, elem ...string) string {
	base := filepath.Clean(cwd)
	if flat && strings.TrimSpace(instructionRoot) != "" {
		base = filepath.Clean(instructionRoot)
	} else if !flat && strings.TrimSpace(instructionRoot) == "" {
		// Repo-style overlay: session files under <cwd>/.oneclaw/ when that directory exists.
		dot := filepath.Join(base, DotDir)
		if st, err := os.Stat(dot); err == nil && st.IsDir() {
			base = dot
		}
	}
	rel := SessionWorkspaceRel(flat, elem...)
	if rel == "" {
		return base
	}
	return filepath.Join(base, rel)
}
