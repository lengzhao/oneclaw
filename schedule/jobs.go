package schedule

import (
	"strings"
)

// File is the on-disk JSON envelope for scheduled jobs.
type File struct {
	Version int   `json:"version"`
	Jobs    []Job `json:"jobs"`
}

// Job is one persisted schedule entry.
type Job struct {
	ID          string `json:"id"`
	Name        string `json:"name,omitempty"`
	Enabled     bool   `json:"enabled"`
	Kind        string `json:"kind"` // "once" | "cron"
	CronExpr    string `json:"cron_expr,omitempty"`
	NextRunUnix int64  `json:"next_run_unix"`
	// SessionSegment is the raw channel session key (e.g. Weixin …@im.wechat). Persist as-is; sanitize only for paths.
	SessionSegment string            `json:"session_segment"`
	ClientID       string            `json:"client_id,omitempty"`
	Prompt         string            `json:"prompt"`
	AgentID        string            `json:"agent_id,omitempty"`
	ReplyMeta      map[string]string `json:"reply_meta,omitempty"`
}

const (
	KindOnce = "once"
	KindCron = "cron"
)

// Normalize fills defaults on j.
func (j *Job) Normalize() {
	j.Kind = strings.TrimSpace(strings.ToLower(j.Kind))
	if j.Kind == "" {
		j.Kind = KindOnce
	}
	j.ID = strings.TrimSpace(j.ID)
	j.Name = strings.TrimSpace(j.Name)
	j.SessionSegment = strings.TrimSpace(j.SessionSegment)
	j.Prompt = strings.TrimSpace(j.Prompt)
	j.AgentID = strings.TrimSpace(j.AgentID)
	j.ClientID = strings.TrimSpace(j.ClientID)
	j.CronExpr = strings.TrimSpace(j.CronExpr)
	if len(j.ReplyMeta) > 0 {
		out := make(map[string]string)
		for k, v := range j.ReplyMeta {
			kk := strings.TrimSpace(k)
			vv := strings.TrimSpace(v)
			if kk != "" && vv != "" {
				out[kk] = vv
			}
		}
		if len(out) == 0 {
			j.ReplyMeta = nil
		} else {
			j.ReplyMeta = out
		}
	}
}
