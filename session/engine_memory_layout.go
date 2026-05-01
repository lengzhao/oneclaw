package session

import "github.com/lengzhao/oneclaw/workspace"

// MemoryLayout returns file paths for this engine (shared IM root vs per-session root vs repo layout).
func (e *Engine) MemoryLayout(home string) workspace.Layout {
	if e == nil {
		return workspace.Layout{}
	}
	return workspace.LayoutForIMWorkspace(e.CWD, home, e.UserDataRoot, e.WorkspaceFlat, e.InstructionRoot)
}
