package subagent

import (
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

// RegisterDepsBoundBuiltin registers builtins that need [RunAgentDeps] at bind time (e.g. cron).
// Returns handled=false if name is not a deps-bound builtin.
func RegisterDepsBoundBuiltin(out *tools.Registry, name string, deps *RunAgentDeps) (handled bool, err error) {
	switch name {
	case builtin.NameCron:
		if deps == nil {
			return true, fmt.Errorf("subagent: cron requires RunAgentDeps")
		}
		if deps.Cfg != nil && !deps.Cfg.BuiltinToolEnabled(builtin.NameCron) {
			return true, fmt.Errorf("subagent: tool %q is disabled in config", name)
		}
		jobsPath := paths.ScheduledJobsPath(strings.TrimSpace(deps.UserDataRoot))
		tool, err := builtin.InferCron(builtin.CronDeps{
			JobsPath: jobsPath,
			Scope:    scheduleScopeFromTurn(deps.Turn),
		})
		if err != nil {
			return true, err
		}
		return true, out.Register(tool)
	default:
		return false, nil
	}
}

func scheduleScopeFromTurn(t TurnBinding) schedule.JobBindingScope {
	return schedule.JobBindingScope{
		SessionSegment: t.SessionSegment,
		ClientID:       t.InboundClientID,
		AgentID:        t.AgentID,
	}
}
