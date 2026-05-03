package builtin

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/lengzhao/oneclaw/schedule"
)

// CronDeps binds the cron tool to the persisted jobs file and scoped session/channel/agent identity.
type CronDeps struct {
	JobsPath  string
	Scope     schedule.JobBindingScope
	ReplyMeta map[string]string
}

type cronScheduleIn struct {
	AtSeconds    int    `json:"at_seconds,omitempty" jsonschema:"description=One-shot: seconds from now (minimum 10)"`
	AtRFC3339    string `json:"at_rfc3339,omitempty" jsonschema:"description=One-shot: RFC3339 wall time at least 10 seconds in the future"`
	EverySeconds int    `json:"every_seconds,omitempty" jsonschema:"description=Recurring: interval in seconds via @every (minimum 10)"`
	CronExpr     string `json:"cron_expr,omitempty" jsonschema:"description=Recurring: 5-field cron or descriptor such as @every 1h"`
}

type cronToolIn struct {
	Action   string          `json:"action" jsonschema:"description=add | list | remove,required=true"`
	Message  string          `json:"message,omitempty" jsonschema:"description=add: instruction text stored for when the job fires (delivered as a scheduler turn, not live user typing)"`
	Name     string          `json:"name,omitempty" jsonschema:"description=add: optional label"`
	Schedule *cronScheduleIn `json:"schedule,omitempty" jsonschema:"description=add: exactly one nested schedule field"`
	JobID    string          `json:"job_id,omitempty" jsonschema:"description=remove: job id from list"`
}

// InferCron builds the cron builtin (agents persist jobs; serve poller delivers due jobs — no separate cron HTTP API).
func InferCron(d CronDeps) (tool.InvokableTool, error) {
	path := strings.TrimSpace(d.JobsPath)
	if path == "" {
		return nil, fmt.Errorf("%s: jobs path required", NameCron)
	}
	sess := strings.TrimSpace(d.Scope.SessionSegment)
	scope := d.Scope
	return utils.InferTool(NameCron,
		"Cron-style reminders: add/list/remove jobs on disk scoped to this session, clawbridge client id, and catalog agent. "+
			"Schedule granularity is at least 10 seconds (at_seconds, every_seconds, RFC3339 lead time). "+
			"For add, pass schedule with exactly one of at_seconds, at_rfc3339, every_seconds, or cron_expr. "+
			"list/remove only affect jobs created in the same scope. Stop recurring jobs with remove + job_id.",
		func(ctx context.Context, in cronToolIn) (string, error) {
			switch strings.ToLower(strings.TrimSpace(in.Action)) {
			case "add":
				if sess == "" {
					return "", fmt.Errorf("cron: session segment required for add")
				}
				if in.Schedule == nil {
					return "", fmt.Errorf("cron: add requires schedule with exactly one of at_seconds, at_rfc3339, every_seconds, or cron_expr")
				}
				s := in.Schedule
				return schedule.AddScheduleJob(path, schedule.ToolAddInput{
					Name:           in.Name,
					Message:        in.Message,
					SessionSegment: sess,
					ClientID:       strings.TrimSpace(scope.ClientID),
					AgentID:        strings.TrimSpace(scope.AgentID),
					AtSeconds:      s.AtSeconds,
					AtRFC3339:      s.AtRFC3339,
					EverySeconds:   s.EverySeconds,
					CronExpr:       s.CronExpr,
					ReplyMeta:      maps.Clone(d.ReplyMeta),
				})
			case "list":
				return schedule.ListScheduleJobsText(path, scope)
			case "remove":
				return schedule.RemoveScheduleJob(path, in.JobID, scope)
			default:
				return "", fmt.Errorf("unknown action %q", in.Action)
			}
		})
}
