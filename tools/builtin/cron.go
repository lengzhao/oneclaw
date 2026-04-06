package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type cronScheduleIn struct {
	AtSeconds    int    `json:"at_seconds"`
	AtRFC3339    string `json:"at_rfc3339"`
	EverySeconds int    `json:"every_seconds"`
	CronExpr     string `json:"cron_expr"`
}

type cronToolInput struct {
	Action   string          `json:"action"`
	Message  string          `json:"message"`
	Name     string          `json:"name"`
	Schedule *cronScheduleIn `json:"schedule"`
	JobID    string          `json:"job_id"`
}

// CronTool manages persisted scheduled user prompts (see <cwd>/.oneclaw/scheduled_jobs.json).
// Name is `cron` so it is easy to recognize; simplified vs picoclaw: no shell commands—fires by injecting a user message on the matching channel instance.
type CronTool struct{}

func (CronTool) Name() string          { return "cron" }
func (CronTool) ConcurrencySafe() bool { return false }

func (CronTool) Description() string {
	return "Cron-style reminders: add/list/remove jobs on disk. For add, pass `schedule` with exactly one key: at_seconds (from now), at_rfc3339 (one-shot), every_seconds (interval), or cron_expr (5-field). " +
		"Channel/thread bind from runtime. Stop recurring jobs with remove + job_id."
}

func (CronTool) Parameters() openai.FunctionParameters {
	schedProps := map[string]any{
		"at_seconds": map[string]any{
			"type":        "integer",
			"description": "One-shot: seconds from now (e.g. 600)",
		},
		"at_rfc3339": map[string]any{
			"type":        "string",
			"description": "One-shot: RFC3339 wall time (must be in the future)",
		},
		"every_seconds": map[string]any{
			"type":        "integer",
			"description": "Recurring: interval in seconds (e.g. 3600)",
		},
		"cron_expr": map[string]any{
			"type":        "string",
			"description": "Recurring: standard 5-field cron",
		},
	}
	return objectSchema(map[string]any{
		"action": map[string]any{
			"type":        "string",
			"description": "add | list | remove",
			"enum":        []string{"add", "list", "remove"},
		},
		"message": map[string]any{
			"type":        "string",
			"description": "add: user message text when the job fires",
		},
		"name": map[string]any{
			"type":        "string",
			"description": "add: optional label (default: truncated message)",
		},
		"schedule": map[string]any{
			"type":        "object",
			"description": "add: exactly one of the nested keys below",
			"properties":  schedProps,
		},
		"job_id": map[string]any{
			"type":        "string",
			"description": "remove: job id from list",
		},
	}, []string{"action"})
}

func (CronTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in cronToolInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(in.Action)) {
	case "add":
		if tctx == nil {
			return "", fmt.Errorf("cron: missing tool context")
		}
		if in.Schedule == nil {
			return "", fmt.Errorf("cron: add requires schedule with exactly one of at_seconds, at_rfc3339, every_seconds, cron_expr")
		}
		s := in.Schedule
		spec := schedule.ScheduleSpec{
			AtRFC3339:    strings.TrimSpace(s.AtRFC3339),
			EverySeconds: s.EverySeconds,
			CronExpr:     strings.TrimSpace(s.CronExpr),
		}
		ts := strings.TrimSpace(tctx.TurnInbound.Source)
		sk := strings.TrimSpace(tctx.TurnInbound.SessionKey)
		uid := strings.TrimSpace(tctx.TurnInbound.UserID)
		ten := strings.TrimSpace(tctx.TurnInbound.TenantID)
		return schedule.Add(tctx.CWD, schedule.AddInput{
			Name:          in.Name,
			Message:       in.Message,
			TargetSource:  ts,
			SessionKey:    sk,
			UserID:        uid,
			TenantID:      ten,
			Schedule:      spec,
			AtSeconds:     s.AtSeconds,
		})
	case "list":
		return schedule.ListText(tctx.CWD)
	case "remove":
		return schedule.Remove(tctx.CWD, in.JobID)
	default:
		return "", fmt.Errorf("unknown action %q", in.Action)
	}
}
