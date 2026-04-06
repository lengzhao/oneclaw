package builtin

import "github.com/lengzhao/oneclaw/tools"

// DefaultRegistry registers read/write/grep/bash plus glob, list_dir, and subagent tools.
func DefaultRegistry() *tools.Registry {
	r := tools.NewRegistry()
	r.MustRegister(ReadTool{})
	r.MustRegister(WriteTool{})
	r.MustRegister(WriteBehaviorPolicyTool{})
	r.MustRegister(GrepTool{})
	r.MustRegister(GlobTool{})
	r.MustRegister(ListDirTool{})
	r.MustRegister(BashTool{})
	r.MustRegister(RunAgentTool{})
	r.MustRegister(ForkContextTool{})
	r.MustRegister(InvokeSkillTool{})
	r.MustRegister(TaskCreateTool{})
	r.MustRegister(TaskUpdateTool{})
	r.MustRegister(CronTool{})
	r.MustRegister(SendMessageTool{})
	return r
}

// ScheduledMaintainReadRegistry registers read-only tools for memory.RunScheduledMaintain (far-field agent).
// Lives in builtin so memory does not import this package (import cycle: builtin → memory).
func ScheduledMaintainReadRegistry() *tools.Registry {
	r := tools.NewRegistry()
	r.MustRegister(ReadTool{})
	r.MustRegister(GrepTool{})
	r.MustRegister(GlobTool{})
	r.MustRegister(ListDirTool{})
	return r
}
