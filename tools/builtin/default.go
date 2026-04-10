package builtin

import "github.com/lengzhao/oneclaw/tools"

// DefaultRegistry registers read/write/grep/exec plus glob, list_dir, and subagent tools.
// Policy-scoped writes (AGENT.md / .oneclaw/rules / skills) use write_behavior_policy only in ScheduledMaintainReadRegistry, not here.
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
// read-only file tools plus write_behavior_policy (cwd-only: .oneclaw/AGENT.md, .oneclaw/rules, .oneclaw/skills, project MEMORY.md).
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
