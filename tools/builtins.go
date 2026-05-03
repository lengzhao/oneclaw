package tools

import (
	"github.com/cloudwego/eino/components/tool"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

// IsBuiltinToolName reports whether name is a builtin handled by RegisterBuiltinsNamed when wiring sub-agents (workspace rebound).
func IsBuiltinToolName(name string) bool {
	return builtin.IsBuiltinName(name)
}

func dedupeAllow(names []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, n := range names {
		if n == "" || seen[n] {
			continue
		}
		seen[n] = true
		out = append(out, n)
	}
	return out
}

// RegisterBuiltins registers all tools in [DefaultBuiltinIDs] (ignores config; for tests and narrow callers).
func RegisterBuiltins(r *Registry) error {
	return RegisterBuiltinsNamed(r, nil)
}

// RegisterBuiltinsForConfig registers builtins allowed by [config.File.BuiltinToolEnabled].
// Nil cfg behaves like all tools enabled.
func RegisterBuiltinsForConfig(r *Registry, cfg *config.File) error {
	var names []string
	for _, id := range DefaultBuiltinIDs {
		if cfg.BuiltinToolEnabled(id) {
			names = append(names, id)
		}
	}
	return RegisterBuiltinsNamed(r, names)
}

// RegisterBuiltinsNamed registers builtins by id. Empty names registers all [DefaultBuiltinIDs].
func RegisterBuiltinsNamed(r *Registry, names []string) error {
	want := make(map[string]bool)
	var order []string
	if len(names) == 0 {
		order = DefaultBuiltinIDs
		for _, id := range DefaultBuiltinIDs {
			want[id] = true
		}
	} else {
		order = dedupeAllow(names)
		for _, id := range order {
			if builtin.IsBuiltinName(id) {
				want[id] = true
			}
		}
	}

	ws := r.WorkspaceRoot()
	for _, id := range order {
		if !want[id] {
			continue
		}
		var t tool.InvokableTool
		var err error
		switch id {
		case builtin.NameEcho:
			t, err = builtin.InferEcho()
		case builtin.NameReadFile:
			t, err = builtin.InferReadFile(ws)
		case builtin.NameListDir:
			t, err = builtin.InferListDir(ws)
		case builtin.NameGlob:
			t, err = builtin.InferGlob(ws)
		case builtin.NameWriteFile:
			t, err = builtin.InferWriteFile(ws)
		case builtin.NameEditFile:
			t, err = builtin.InferEditFile(ws)
		case builtin.NameAppendFile:
			t, err = builtin.InferAppendFile(ws)
		case builtin.NameExec:
			t, err = builtin.InferExec(ws)
		default:
			continue
		}
		if err != nil {
			return err
		}
		if err := r.Register(t); err != nil {
			return err
		}
	}
	return nil
}
