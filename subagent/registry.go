package subagent

import (
	"fmt"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/toolhost"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

// BuildRegistryForAgent builds a tool registry for the root agent (parent==nil) or a sub-agent.
// Root: allow empty means all non-meta builtins from RegisterBuiltins; run_agent is included only if listed in allow.
// Child: allow empty means DefaultSubagentToolTemplate intersected with parent names (narrow default; not parent full set).
func BuildRegistryForAgent(ws string, allow []string, parent toolhost.Registry, deps *RunAgentDeps) (*tools.Registry, error) {
	if parent == nil {
		return buildRootRegistry(ws, allow, deps)
	}
	return buildChildRegistry(parent, ws, allow, deps)
}

// BuildExecRegistry is an alias for the root-agent case (parent nil).
func BuildExecRegistry(ws string, allow []string, deps *RunAgentDeps) (*tools.Registry, error) {
	return BuildRegistryForAgent(ws, allow, nil, deps)
}

// ChildRegistryFromParent preserves the phase-4 helper name for tests and callers.
func ChildRegistryFromParent(parent toolhost.Registry, childWS string, allow []string, runTmpl *RunAgentDeps) (*tools.Registry, error) {
	return buildChildRegistry(parent, childWS, allow, runTmpl)
}

func buildRootRegistry(ws string, allow []string, deps *RunAgentDeps) (*tools.Registry, error) {
	if deps == nil {
		return nil, fmt.Errorf("subagent: root registry requires RunAgentDeps")
	}
	seed := tools.NewRegistry(ws)
	if err := tools.RegisterBuiltinsForConfig(seed, deps.Cfg); err != nil {
		return nil, err
	}
	names := normalizeRootAllow(allow, seed, deps.Cfg)
	return assembleRegistry(ws, names, seed, nil, deps)
}

func buildChildRegistry(parent toolhost.Registry, childWS string, allow []string, deps *RunAgentDeps) (*tools.Registry, error) {
	names := dedupeAllow(allow)
	if len(names) == 0 {
		names = intersectParentTemplate(parent, DefaultSubagentToolTemplate)
		if len(names) == 0 {
			return nil, fmt.Errorf("subagent: parent registry has none of default sub-agent template %v (FR-AGT-03 subset)",
				DefaultSubagentToolTemplate)
		}
	}
	return assembleRegistry(childWS, names, nil, parent, deps)
}

func normalizeRootAllow(allow []string, seed *tools.Registry, cfg *config.File) []string {
	if len(allow) == 0 {
		var out []string
		for _, n := range seed.Names() {
			if !IsMetaToolName(n) {
				out = append(out, n)
			}
		}
		if cfg == nil || cfg.BuiltinToolEnabled(builtin.NameCron) {
			out = append(out, builtin.NameCron)
		}
		return dedupeAllow(out)
	}
	return dedupeAllow(allow)
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

func assembleRegistry(ws string, names []string, seed *tools.Registry, parent toolhost.Registry, deps *RunAgentDeps) (*tools.Registry, error) {
	out := tools.NewRegistry(ws)
	wantRA := false
	for _, n := range names {
		if IsMetaToolName(n) {
			if n == "run_agent" {
				wantRA = true
			}
			continue
		}
		handled, err := RegisterDepsBoundBuiltin(out, n, deps)
		if err != nil {
			return nil, err
		}
		if handled {
			continue
		}
		if seed != nil {
			ts, err := seed.FilterByNames([]string{n})
			if err != nil {
				return nil, fmt.Errorf("subagent: unknown tool %q", n)
			}
			if err := out.Register(ts[0]); err != nil {
				return nil, err
			}
			continue
		}
		if err := copyOrRebindTool(parent, out, ws, n, deps); err != nil {
			return nil, err
		}
	}
	if wantRA {
		if deps == nil {
			return nil, fmt.Errorf("subagent: run_agent requires RunAgentDeps")
		}
		if err := RegisterRunAgent(out, deps); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func copyOrRebindTool(parent toolhost.Registry, child *tools.Registry, childWS, name string, deps *RunAgentDeps) error {
	if !containsToolName(parent.Names(), name) {
		return fmt.Errorf("subagent: tool %q not on parent registry", name)
	}
	handled, err := RegisterDepsBoundBuiltin(child, name, deps)
	if err != nil {
		return err
	}
	if handled {
		return nil
	}
	if tools.IsBuiltinToolName(name) {
		return tools.RegisterBuiltinsNamed(child, []string{name})
	}
	ts, err := parent.FilterByNames([]string{name})
	if err != nil {
		return err
	}
	if len(ts) != 1 {
		return fmt.Errorf("subagent: internal: tool %q missing", name)
	}
	// Non-builtins keep parent bindings; private child workspaces may need explicit rebound factories later.
	return child.Register(ts[0])
}

func containsToolName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}
