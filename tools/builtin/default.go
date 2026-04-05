package builtin

import "github.com/lengzhao/oneclaw/tools"

// DefaultRegistry registers read_file, write_file, grep, and bash.
func DefaultRegistry() *tools.Registry {
	r := tools.NewRegistry()
	r.MustRegister(ReadTool{})
	r.MustRegister(WriteTool{})
	r.MustRegister(GrepTool{})
	r.MustRegister(BashTool{})
	r.MustRegister(RunAgentTool{})
	r.MustRegister(ForkContextTool{})
	return r
}
