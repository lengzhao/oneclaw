package tools

import "github.com/lengzhao/oneclaw/tools/builtin"

// Canonical builtin tool names (re-export [github.com/lengzhao/oneclaw/tools/builtin] constants).
const (
	ToolEcho       = builtin.NameEcho
	ToolReadFile   = builtin.NameReadFile
	ToolListDir    = builtin.NameListDir
	ToolGlob       = builtin.NameGlob
	ToolWriteFile  = builtin.NameWriteFile
	ToolEditFile   = builtin.NameEditFile
	ToolAppendFile = builtin.NameAppendFile
	ToolExec       = builtin.NameExec
	ToolCron       = builtin.NameCron
)

// EssentialWorkspaceToolIDs lists builtins an agent needs to consume files under WorkspaceRoot.
var EssentialWorkspaceToolIDs = []string{builtin.NameReadFile}

// DefaultBuiltinIDs is registration order for [RegisterBuiltins] (empty names → all).
var DefaultBuiltinIDs = builtin.DefaultRegistrationOrder

// DefaultSubagentBuiltinIDs is the narrow template intersected with the parent registry (read-heavy; no write by default).
var DefaultSubagentBuiltinIDs = builtin.DefaultSubagentOrder
