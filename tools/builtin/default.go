package builtin

import "github.com/lengzhao/oneclaw/tools"

// DefaultRegistry registers read/write/grep/exec plus glob, list_dir, and subagent tools.
// Policy-scoped writes (AGENT.md / rules / skills / MEMORY.md) use write_behavior_policy.
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
