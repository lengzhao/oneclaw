package builtin

// DefaultRegistrationOrder is used when [github.com/lengzhao/oneclaw/tools.RegisterBuiltinsNamed] receives empty names.
var DefaultRegistrationOrder = []string{
	NameReadFile,
	NameListDir,
	NameGlob,
	NameWriteFile,
	NameEditFile,
	NameAppendFile,
	NameExec,
	NameCron,
	NameTodo,
}

// DefaultSubagentOrder is the narrow child-agent template (intersected with parent registry).
var DefaultSubagentOrder = []string{NameReadFile, NameListDir}

// IsBuiltinName reports names understood by the builtin registration path (including sub-agent rebind).
func IsBuiltinName(name string) bool {
	switch name {
	case NameEcho, NameReadFile, NameListDir, NameGlob, NameWriteFile, NameEditFile, NameAppendFile, NameExec, NameCron, NameTodo:
		return true
	default:
		return false
	}
}
