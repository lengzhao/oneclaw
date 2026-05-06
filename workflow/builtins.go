package workflow

// BuiltinUses lists every built-in workflow node kind implemented by wfexec.RegisterBuiltins.
// This slice is the single source of truth; AllowedUses is derived in init for validation (tests may add transient keys).
var BuiltinUses = []string{
	"on_receive",
	"llm",
	"on_respond",
	"agent_task",
	"structured_memory_extract",
	"retrieve_context",
	"command",
	"tool_call",
	"noop",
}

// AllowedUses is the validation whitelist populated from BuiltinUses.
var AllowedUses map[string]struct{}

func init() {
	AllowedUses = make(map[string]struct{}, len(BuiltinUses)+16)
	for _, u := range BuiltinUses {
		AllowedUses[u] = struct{}{}
	}
}
