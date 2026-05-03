package config

import "strings"

// BuiltinToolExec is the config/tools YAML key for the exec builtin (default off).
const BuiltinToolExec = "exec"

// ToolSwitch toggles a builtin tool (YAML under tools.<tool_name>).
// Allow/Deny apply to [BuiltinToolExec] only (prefix allowlist / substring denials).
// Allow entry "*" means any command (still blocked by Deny substrings).
type ToolSwitch struct {
	// Enabled defaults to true when omitted for normal tools; exec requires explicit enabled: true (see [File.BuiltinToolEnabled]).
	Enabled *bool `yaml:"enabled,omitempty"`
	Allow   []string `yaml:"allow,omitempty"`
	Deny    []string `yaml:"deny,omitempty"`
}

// BuiltinToolEnabled reports whether a builtin id may be registered at the root registry.
// Exec is opt-in: absent or unset enabled means off. Other tools default on when omitted.
func (f *File) BuiltinToolEnabled(toolName string) bool {
	if toolName == BuiltinToolExec {
		if f == nil || len(f.Tools) == 0 {
			return false
		}
		sw, ok := f.Tools[BuiltinToolExec]
		if !ok {
			return false
		}
		if sw.Enabled == nil {
			return false
		}
		return *sw.Enabled
	}
	if f == nil || len(f.Tools) == 0 {
		return true
	}
	sw, ok := f.Tools[toolName]
	if !ok {
		return true
	}
	if sw.Enabled == nil {
		return true
	}
	return *sw.Enabled
}

// ExecCommandPermitted checks runtime policy for a shell command string (enable + deny + allow-prefix).
func (f *File) ExecCommandPermitted(command string) bool {
	if !f.BuiltinToolEnabled(BuiltinToolExec) {
		return false
	}
	sw, ok := f.Tools[BuiltinToolExec]
	if !ok {
		return false
	}
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return false
	}
	for _, d := range sw.Deny {
		ds := strings.TrimSpace(d)
		if ds != "" && strings.Contains(cmd, ds) {
			return false
		}
	}
	if len(sw.Allow) == 0 {
		return false
	}
	for _, a := range sw.Allow {
		ap := strings.TrimSpace(a)
		if ap == "*" {
			return true
		}
		if ap != "" && strings.HasPrefix(cmd, ap) {
			return true
		}
	}
	return false
}

// ExecCommandAllowedFromRuntime uses [Runtime] snapshot (must call after [PushRuntime]).
func ExecCommandAllowedFromRuntime(command string) bool {
	rv := Runtime()
	if rv == nil || rv.Config == nil {
		return false
	}
	return rv.Config.ExecCommandPermitted(command)
}
