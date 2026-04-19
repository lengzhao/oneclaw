package session

import "github.com/lengzhao/oneclaw/memory"

// MemoryLayout returns file-memory paths for this engine (shared IM root vs per-session root vs repo layout).
func (e *Engine) MemoryLayout(home string) memory.Layout {
	if e == nil {
		return memory.Layout{}
	}
	return memory.LayoutForIMWorkspace(e.CWD, home, e.UserDataRoot, e.WorkspaceFlat, e.InstructionRoot)
}
