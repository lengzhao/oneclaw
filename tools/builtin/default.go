package builtin

import "github.com/lengzhao/oneclaw/tools"

// DefaultRegistry registers read/write/grep/bash plus glob, list_dir, and subagent tools.
func DefaultRegistry() *tools.Registry {
	r := tools.NewRegistry()
	r.MustRegister(ReadTool{})
	r.MustRegister(WriteTool{})
	r.MustRegister(GrepTool{})
	r.MustRegister(GlobTool{})
	r.MustRegister(ListDirTool{})
	r.MustRegister(BashTool{})
	r.MustRegister(RunAgentTool{})
	r.MustRegister(ForkContextTool{})
	r.MustRegister(InvokeSkillTool{})
	return r
}
