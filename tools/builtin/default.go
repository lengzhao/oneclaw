package builtin

import "github.com/lengzhao/oneclaw/tools"

// DefaultRegistry registers read/write/grep/exec plus glob, list_dir, and subagent tools.
// Policy-scoped writes (AGENT.md / rules / skills) use write_behavior_policy only in ScheduledMaintainReadRegistry, not here.
func DefaultRegistry() *tools.Registry {
	r := tools.NewRegistry()
	r.MustRegister(ReadTool{})
	r.MustRegister(WriteTool{})
	r.MustRegister(GrepTool{})
	r.MustRegister(GlobTool{})
	r.MustRegister(ListDirTool{})
	r.MustRegister(ExecTool{})
	r.MustRegister(RunAgentTool{})
	r.MustRegister(ForkContextTool{})
	r.MustRegister(InvokeSkillTool{})
	r.MustRegister(TaskCreateTool{})
	r.MustRegister(TaskUpdateTool{})
	r.MustRegister(CronTool{})
	r.MustRegister(SendMessageTool{})
	return r
}

// ScheduledMaintainReadRegistry registers tools for memory.RunScheduledMaintain (far-field agent):
// read-only file tools plus write_behavior_policy (AGENT.md, rules, skills, project MEMORY.md, plus agent_memory with scope+agent_type).
// Lives in builtin so memory does not import this package (import cycle: builtin → memory).
func ScheduledMaintainReadRegistry() *tools.Registry {
	r := tools.NewRegistry()
	r.MustRegister(ReadTool{})
	r.MustRegister(GrepTool{})
	r.MustRegister(GlobTool{})
	r.MustRegister(ListDirTool{})
	r.MustRegister(WriteBehaviorPolicyTool{})
	return r
}
