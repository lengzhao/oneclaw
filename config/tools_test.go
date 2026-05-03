package config

import (
	"testing"
)

func TestFile_BuiltinToolEnabled(t *testing.T) {
	if (*File)(nil).BuiltinToolEnabled("glob") != true {
		t.Fatal("nil File should allow non-exec builtins")
	}
	if (*File)(nil).BuiltinToolEnabled(BuiltinToolExec) {
		t.Fatal("nil File must keep exec off")
	}
	f := &File{}
	if !f.BuiltinToolEnabled("write_file") {
		t.Fatal("empty tools map defaults on for normal tools")
	}
	if f.BuiltinToolEnabled(BuiltinToolExec) {
		t.Fatal("exec must stay off without explicit config")
	}
	disabled := false
	f.Tools = map[string]ToolSwitch{
		"glob": {Enabled: &disabled},
	}
	if f.BuiltinToolEnabled("glob") {
		t.Fatal("glob should be off")
	}
	if !f.BuiltinToolEnabled("read_file") {
		t.Fatal("unspecified tool stays on")
	}
}

func TestFile_ExecCommandPermitted(t *testing.T) {
	en := true
	f := &File{
		Tools: map[string]ToolSwitch{
			BuiltinToolExec: {
				Enabled: &en,
				Allow:   []string{"echo ", "git "},
				Deny:    []string{";"},
			},
		},
	}
	if !f.ExecCommandPermitted("echo hi") {
		t.Fatal("echo hi should match allow prefix")
	}
	if f.ExecCommandPermitted("git status; rm") {
		t.Fatal("deny should win on substring")
	}
	if f.ExecCommandPermitted("rm -rf") {
		t.Fatal("rm should not match allow")
	}
	off := false
	f.Tools[BuiltinToolExec] = ToolSwitch{Enabled: &off, Allow: []string{"echo "}}
	if f.ExecCommandPermitted("echo hi") {
		t.Fatal("disabled exec")
	}
}

func TestFile_ExecCommandPermitted_wildcardAllow(t *testing.T) {
	en := true
	f := &File{
		Tools: map[string]ToolSwitch{
			BuiltinToolExec: {
				Enabled: &en,
				Allow:   []string{"*"},
				Deny:    []string{";"},
			},
		},
	}
	if !f.ExecCommandPermitted("any-command here") {
		t.Fatal("wildcard allow")
	}
	if f.ExecCommandPermitted("name; inj") {
		t.Fatal("deny should still apply")
	}
}
