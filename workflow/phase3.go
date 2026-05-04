package workflow

// Phase3BuiltinUses lists every built-in workflow node kind implemented by wfexec.RegisterPhase3Builtins.
// This slice is the single source of truth; Phase3Uses is derived in init for validation (tests may add transient keys).
var Phase3BuiltinUses = []string{
	"on_receive",
	"load_prompt_md",
	"load_memory_snapshot",
	"list_skills",
	"list_tasks",
	"load_transcript",
	"filter_tools",
	"adk_main",
	"on_respond",
	"agent",
	"noop",
}

// Phase3Uses is the validation whitelist populated from Phase3BuiltinUses.
var Phase3Uses map[string]struct{}

func init() {
	Phase3Uses = make(map[string]struct{}, len(Phase3BuiltinUses)+16)
	for _, u := range Phase3BuiltinUses {
		Phase3Uses[u] = struct{}{}
	}
}
