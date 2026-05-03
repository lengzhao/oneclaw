package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

const maxScheduleJobs = 64

// ToolAddInput is validated input for [AddScheduleJob] (cron tool add action).
type ToolAddInput struct {
	Name           string
	Message        string
	SessionSegment string
	ClientID       string
	AgentID        string
	AtSeconds      int
	AtRFC3339      string
	EverySeconds   int
	CronExpr       string
}

func newScheduleJobID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "sj_" + hex.EncodeToString(b[:])
}

func truncateStr(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func mergeScheduleKinds(atSeconds int, atRFC3339 string, everySeconds int, cronExpr string) (kind string, nextUnix int64, cronOut string, err error) {
	hasSec := atSeconds > 0
	hasAt := strings.TrimSpace(atRFC3339) != ""
	hasEvery := everySeconds > 0
	hasCron := strings.TrimSpace(cronExpr) != ""
	n := 0
	if hasSec {
		n++
	}
	if hasAt {
		n++
	}
	if hasEvery {
		n++
	}
	if hasCron {
		n++
	}
	if n != 1 {
		return "", 0, "", fmt.Errorf("set exactly one of at_seconds (>0), at_rfc3339, every_seconds (>0), or cron_expr")
	}
	now := time.Now().UTC()
	nowUnix := now.Unix()
	if hasSec {
		return KindOnce, now.Add(time.Duration(atSeconds) * time.Second).Unix(), "", nil
	}
	if hasAt {
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(atRFC3339))
		if err != nil {
			return "", 0, "", fmt.Errorf("at_rfc3339: %w", err)
		}
		if !t.After(now.Add(-time.Second)) {
			return "", 0, "", fmt.Errorf("at_rfc3339 must be in the future")
		}
		return KindOnce, t.UTC().Unix(), "", nil
	}
	if hasEvery {
		if everySeconds < 1 {
			return "", 0, "", fmt.Errorf("every_seconds must be >= 1")
		}
		expr := fmt.Sprintf("@every %ds", everySeconds)
		sch, err := parseCronSchedule(expr)
		if err != nil {
			return "", 0, "", fmt.Errorf("every_seconds: %w", err)
		}
		next := sch.Next(now)
		nextU := applyMinFireGap(next.UTC().Unix(), nowUnix)
		return KindCron, nextU, expr, nil
	}
	expr := strings.TrimSpace(cronExpr)
	sch, err := parseCronSchedule(expr)
	if err != nil {
		return "", 0, "", fmt.Errorf("cron_expr: %w", err)
	}
	next := sch.Next(now)
	nextU := applyMinFireGap(next.UTC().Unix(), nowUnix)
	return KindCron, nextU, expr, nil
}

// AddScheduleJob appends a job to scheduled_jobs.json under path.
func AddScheduleJob(path string, in ToolAddInput) (string, error) {
	msg := strings.TrimSpace(in.Message)
	if msg == "" {
		return "", fmt.Errorf("message is required")
	}
	seg := strings.TrimSpace(in.SessionSegment)
	if seg == "" {
		return "", fmt.Errorf("session segment is required")
	}
	kind, nextUnix, cronExpr, err := mergeScheduleKinds(in.AtSeconds, in.AtRFC3339, in.EverySeconds, in.CronExpr)
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = truncateStr(msg, 40)
	}
	var summary string
	err = mutateJobsFile(path, func(f *File) (bool, error) {
		if len(f.Jobs) >= maxScheduleJobs {
			return false, fmt.Errorf("too many scheduled jobs (max %d)", maxScheduleJobs)
		}
		id := newScheduleJobID()
		job := Job{
			ID:             id,
			Name:           name,
			Enabled:        true,
			Kind:           kind,
			CronExpr:       cronExpr,
			NextRunUnix:    nextUnix,
			SessionSegment: seg,
			ClientID:       strings.TrimSpace(in.ClientID),
			Prompt:         msg,
			AgentID:        strings.TrimSpace(in.AgentID),
		}
		job.Normalize()
		f.Jobs = append(f.Jobs, job)
		nextT := time.Unix(nextUnix, 0).UTC()
		summary = fmt.Sprintf("scheduled job %s (%q) kind=%s next_run=%s path=%s", id, name, kind, nextT.Format(time.RFC3339), path)
		slog.Info("schedule.job_added", "id", id, "name", name, "kind", kind, "next_run", nextT.Format(time.RFC3339), "path", path)
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return summary, nil
}

// RemoveScheduleJob deletes a job by id visible under scope (same session, client, agent as list).
func RemoveScheduleJob(path, jobID string, scope JobBindingScope) (string, error) {
	id := strings.TrimSpace(jobID)
	if id == "" {
		return "", fmt.Errorf("job_id is required")
	}
	var summary string
	err := mutateJobsFile(path, func(f *File) (bool, error) {
		var out []Job
		found := false
		for _, j := range f.Jobs {
			if j.ID == id {
				if !JobMatchesScope(j, scope) {
					return false, fmt.Errorf("unknown job_id %q", id)
				}
				found = true
				continue
			}
			out = append(out, j)
		}
		if !found {
			return false, fmt.Errorf("unknown job_id %q", id)
		}
		f.Jobs = out
		summary = fmt.Sprintf("removed scheduled job %s", id)
		slog.Info("schedule.job_removed", "id", id, "path", path)
		return true, nil
	})
	if err != nil {
		return "", err
	}
	return summary, nil
}

// ListScheduleJobsText returns a human-readable list of enabled jobs visible under scope.
func ListScheduleJobsText(path string, scope JobBindingScope) (string, error) {
	unlock := schedulePathLocks.lock(path)
	defer unlock()
	f, err := Load(path)
	if err != nil {
		slog.Error("schedule.list", "phase", "load", "path", path, "err", err)
		return "", err
	}
	before := len(f.Jobs)
	compactStaleJobs(f)
	if len(f.Jobs) != before {
		if err := Save(path, f); err != nil {
			return "", err
		}
	}
	var b strings.Builder
	n := 0
	for _, j := range f.Jobs {
		j.Normalize()
		if !j.Enabled || !JobMatchesScope(j, scope) {
			continue
		}
		n++
		next := time.Unix(j.NextRunUnix, 0).UTC().Format(time.RFC3339)
		nm := j.Name
		if nm == "" {
			nm = "(unnamed)"
		}
		fmt.Fprintf(&b, "- %s name=%q kind=%s next=%s session=%q client=%q agent=%q prompt=%q\n",
			j.ID, nm, j.Kind, next, j.SessionSegment, j.ClientID, j.AgentID, truncateStr(j.Prompt, 120))
	}
	if n == 0 {
		return "(no scheduled jobs)", nil
	}
	return strings.TrimSuffix(b.String(), "\n"), nil
}
