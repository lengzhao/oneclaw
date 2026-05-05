package subagent

import (
	"fmt"
	"maps"
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
	case builtin.NameReadFile:
		if deps == nil {
			return false, nil
		}
		ws := strings.TrimSpace(deps.ParentWorkspace)
		if ws == "" {
			return true, fmt.Errorf("subagent: %s requires ParentWorkspace", name)
		}
		ir := strings.TrimSpace(deps.InstructionRoot)
		ud := strings.TrimSpace(deps.UserDataRoot)
		tool, err := builtin.InferReadFileScoped(ws, ir, ud)
		if err != nil {
			return true, err
		}
		return true, out.Register(tool)
	case builtin.NameWriteFile:
		if deps == nil {
			return false, nil
		}
		ws := strings.TrimSpace(deps.ParentWorkspace)
		if ws == "" {
			return true, fmt.Errorf("subagent: %s requires ParentWorkspace", name)
		}
		ir := strings.TrimSpace(deps.InstructionRoot)
		ud := strings.TrimSpace(deps.UserDataRoot)
		tool, err := builtin.InferWriteFileScoped(ws, ir, ud)
		if err != nil {
			return true, err
		}
		return true, out.Register(tool)
	case builtin.NameTodo:
		if deps == nil {
			return true, fmt.Errorf("subagent: %s requires RunAgentDeps", name)
		}
		if deps.Cfg != nil && !deps.Cfg.BuiltinToolEnabled(builtin.NameTodo) {
			return true, fmt.Errorf("subagent: tool %q is disabled in config", name)
		}
		ir := strings.TrimSpace(deps.InstructionRoot)
		if ir == "" {
			return true, fmt.Errorf("subagent: %s requires InstructionRoot", name)
		}
		tool, err := builtin.InferTodo(ir)
		if err != nil {
			return true, err
		}
		return true, out.Register(tool)
	case builtin.NameCron:
		if deps == nil {
			return true, fmt.Errorf("subagent: cron requires RunAgentDeps")
		}
		if deps.Cfg != nil && !deps.Cfg.BuiltinToolEnabled(builtin.NameCron) {
			return true, fmt.Errorf("subagent: tool %q is disabled in config", name)
		}
		jobsPath := paths.ScheduledJobsPath(strings.TrimSpace(deps.UserDataRoot))
		tool, err := builtin.InferCron(builtin.CronDeps{
			JobsPath:  jobsPath,
			Scope:     scheduleScopeFromTurn(deps.Turn),
			ReplyMeta: maps.Clone(deps.Turn.ReplyMeta),
		})
		if err != nil {
			return true, err
		}
		return true, out.Register(tool)
	case builtin.NameReadRunJournal:
		if deps == nil {
			return true, fmt.Errorf("subagent: %s requires RunAgentDeps", name)
		}
		sr := strings.TrimSpace(deps.SessionRoot)
		if sr == "" {
			return true, fmt.Errorf("subagent: %s requires SessionRoot", name)
		}
		host := strings.TrimSpace(deps.HostAgentID)
		tool, err := builtin.InferReadRunJournal(sr, host, strings.TrimSpace(deps.CorrelationID))
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
