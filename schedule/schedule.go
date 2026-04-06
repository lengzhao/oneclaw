// Package schedule persists user/agent-defined timed jobs (simplified vs picoclaw: no shell commands).
// Jobs live in <cwd>/.oneclaw/scheduled_jobs.json; a background poller injects due messages as user turns.
package schedule

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/robfig/cron/v3"
)

const fileName = "scheduled_jobs.json"

// Path returns the absolute path to <cwd>/.oneclaw/scheduled_jobs.json.
func Path(cwd string) string {
	return filepath.Join(cwd, memory.DotDir, fileName)
}

// Disabled reports ONCLAW_DISABLE_SCHEDULED_TASKS.
func Disabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_SCHEDULED_TASKS"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// File is the on-disk JSON shape.
type File struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Jobs      []Job     `json:"jobs"`
}

// Job is one scheduled reminder / prompt injection.
type Job struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Message string `json:"message"`
	// Enabled is always true for new jobs; false rows are removed on the next write (list/add/remove/collect) so the job list stays small.
	Enabled      bool         `json:"enabled"`
	TargetSource string       `json:"target_source"`
	SessionKey   string       `json:"session_key,omitempty"`
	UserID       string       `json:"user_id,omitempty"`
	TenantID     string       `json:"tenant_id,omitempty"`
	Schedule     ScheduleSpec `json:"schedule"`
	NextRun      time.Time    `json:"next_run"`
	LastRun      *time.Time   `json:"last_run,omitempty"`
}

// ScheduleSpec defines when to fire (exactly one variant should be set; see Validate).
type ScheduleSpec struct {
	Kind         string `json:"kind"` // at | every | cron
	AtRFC3339    string `json:"at_rfc3339,omitempty"`
	EverySeconds int    `json:"every_seconds,omitempty"`
	CronExpr     string `json:"cron_expr,omitempty"`
}

var fileMu sync.Mutex

func newID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "sj_" + hex.EncodeToString(b[:])
}

// Read loads jobs from path. Missing file yields an empty File (no error).
func Read(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{Version: 1, Jobs: nil}, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return &f, nil
}

func write(path string, f *File) error {
	f.Version = 1
	f.UpdatedAt = time.Now().UTC()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(dir, ".sched-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

const maxJobs = 64

// MergeScheduleInputs requires exactly one of atSeconds (>0), non-empty spec.AtRFC3339,
// spec.EverySeconds > 0, or non-empty spec.CronExpr. Relative at_seconds is converted to RFC3339 then validated.
func MergeScheduleInputs(spec ScheduleSpec, atSeconds int) (ScheduleSpec, error) {
	hasSec := atSeconds > 0
	hasAt := strings.TrimSpace(spec.AtRFC3339) != ""
	hasEvery := spec.EverySeconds > 0
	hasCron := strings.TrimSpace(spec.CronExpr) != ""
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
		return ScheduleSpec{}, fmt.Errorf("set exactly one of at_seconds (>0), at_rfc3339, every_seconds (>0), or cron_expr")
	}
	if hasSec {
		t := time.Now().UTC().Add(time.Duration(atSeconds) * time.Second)
		spec = ScheduleSpec{AtRFC3339: t.Format(time.RFC3339)}
	}
	return spec.Validate()
}

// Validate checks schedule spec and returns kind-normalized copy.
func (s ScheduleSpec) Validate() (ScheduleSpec, error) {
	hasAt := strings.TrimSpace(s.AtRFC3339) != ""
	hasEvery := s.EverySeconds > 0
	hasCron := strings.TrimSpace(s.CronExpr) != ""
	n := 0
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
		return ScheduleSpec{}, fmt.Errorf("set exactly one of at_rfc3339, every_seconds (>0), or cron_expr")
	}
	if hasAt {
		if _, err := time.Parse(time.RFC3339, strings.TrimSpace(s.AtRFC3339)); err != nil {
			return ScheduleSpec{}, fmt.Errorf("at_rfc3339: %w", err)
		}
		out := s
		out.Kind = "at"
		out.AtRFC3339 = strings.TrimSpace(s.AtRFC3339)
		out.EverySeconds = 0
		out.CronExpr = ""
		return out, nil
	}
	if hasEvery {
		out := s
		out.Kind = "every"
		out.AtRFC3339 = ""
		out.CronExpr = ""
		return out, nil
	}
	expr := strings.TrimSpace(s.CronExpr)
	if _, err := cron.ParseStandard(expr); err != nil {
		return ScheduleSpec{}, fmt.Errorf("cron_expr: %w", err)
	}
	out := s
	out.Kind = "cron"
	out.CronExpr = expr
	out.AtRFC3339 = ""
	out.EverySeconds = 0
	return out, nil
}

// NextRunAfter computes the first activation strictly after `from`.
func NextRunAfter(spec ScheduleSpec, from time.Time) (time.Time, error) {
	switch spec.Kind {
	case "at":
		t, err := time.Parse(time.RFC3339, spec.AtRFC3339)
		if err != nil {
			return time.Time{}, err
		}
		return t, nil
	case "every":
		if spec.EverySeconds <= 0 {
			return time.Time{}, fmt.Errorf("every_seconds invalid")
		}
		d := time.Duration(spec.EverySeconds) * time.Second
		return from.Add(d), nil
	case "cron":
		sched, err := cron.ParseStandard(spec.CronExpr)
		if err != nil {
			return time.Time{}, err
		}
		return sched.Next(from), nil
	default:
		return time.Time{}, fmt.Errorf("unknown schedule kind %q", spec.Kind)
	}
}

// initialNextRun is the first fire time (>= now with small skew for "at").
func initialNextRun(spec ScheduleSpec, now time.Time) (time.Time, error) {
	switch spec.Kind {
	case "at":
		t, err := time.Parse(time.RFC3339, spec.AtRFC3339)
		if err != nil {
			return time.Time{}, err
		}
		if !t.After(now.Add(-time.Second)) {
			return time.Time{}, fmt.Errorf("at_rfc3339 must be in the future")
		}
		return t, nil
	case "every":
		return now.Add(time.Duration(spec.EverySeconds) * time.Second), nil
	case "cron":
		sched, err := cron.ParseStandard(spec.CronExpr)
		if err != nil {
			return time.Time{}, err
		}
		n := sched.Next(now)
		if n.IsZero() {
			return time.Time{}, fmt.Errorf("cron expression has no next occurrence")
		}
		return n, nil
	default:
		return time.Time{}, fmt.Errorf("unknown schedule kind %q", spec.Kind)
	}
}

// AddInput is the payload for adding a job.
type AddInput struct {
	Name         string
	Message      string
	TargetSource string
	SessionKey   string
	UserID       string
	TenantID     string
	Schedule     ScheduleSpec
	AtSeconds    int
}

// Add appends a job after validation.
func Add(cwd string, in AddInput) (string, error) {
	if Disabled() {
		return "", fmt.Errorf("scheduled tasks are disabled (ONCLAW_DISABLE_SCHEDULED_TASKS)")
	}
	msg := strings.TrimSpace(in.Message)
	if msg == "" {
		return "", fmt.Errorf("message is required")
	}
	spec, err := MergeScheduleInputs(in.Schedule, in.AtSeconds)
	if err != nil {
		return "", err
	}
	ts := strings.TrimSpace(in.TargetSource)
	if ts == "" {
		ts = "cli"
	}
	path := Path(cwd)
	fileMu.Lock()
	f, err := Read(path)
	if err != nil {
		fileMu.Unlock()
		return "", err
	}
	if rm := compactDisabledJobs(f); rm > 0 {
		slog.Info("schedule.compacted_disabled", "removed", rm, "path", path)
	}
	if len(f.Jobs) >= maxJobs {
		fileMu.Unlock()
		return "", fmt.Errorf("too many scheduled jobs (max %d)", maxJobs)
	}
	now := time.Now()
	next, err := initialNextRun(spec, now)
	if err != nil {
		fileMu.Unlock()
		return "", err
	}
	id := newID()
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = truncate(msg, 40)
	}
	f.Jobs = append(f.Jobs, Job{
		ID:           id,
		Name:         name,
		Message:      msg,
		Enabled:      true,
		TargetSource: ts,
		SessionKey:   strings.TrimSpace(in.SessionKey),
		UserID:       strings.TrimSpace(in.UserID),
		TenantID:     strings.TrimSpace(in.TenantID),
		Schedule:     spec,
		NextRun:      next.UTC(),
	})
	err = write(path, f)
	fileMu.Unlock()
	if err != nil {
		return "", err
	}
	slog.Info("schedule.job_added",
		"id", id,
		"name", name,
		"target", ts,
		"kind", spec.Kind,
		"next_run", next.UTC().Format(time.RFC3339),
		"path", path,
		"message_preview", truncate(msg, 200),
	)
	notifyScheduleWake()
	return fmt.Sprintf("scheduled job %s (%q) next run %s (file %s)", id, name, next.UTC().Format(time.RFC3339), path), nil
}

func truncate(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// compactDisabledJobs removes jobs with Enabled==false from f.Jobs. Returns how many rows were removed.
func compactDisabledJobs(f *File) int {
	n := len(f.Jobs)
	out := f.Jobs[:0]
	for _, j := range f.Jobs {
		if j.Enabled {
			out = append(out, j)
		}
	}
	f.Jobs = out
	return n - len(f.Jobs)
}

// Remove deletes a job by id.
func Remove(cwd, jobID string) (string, error) {
	if Disabled() {
		return "", fmt.Errorf("scheduled tasks are disabled (ONCLAW_DISABLE_SCHEDULED_TASKS)")
	}
	id := strings.TrimSpace(jobID)
	if id == "" {
		return "", fmt.Errorf("job_id is required")
	}
	path := Path(cwd)
	fileMu.Lock()
	f, err := Read(path)
	if err != nil {
		fileMu.Unlock()
		return "", err
	}
	if rm := compactDisabledJobs(f); rm > 0 {
		slog.Info("schedule.compacted_disabled", "removed", rm, "path", path)
	}
	var out []Job
	found := false
	for _, j := range f.Jobs {
		if j.ID == id {
			found = true
			continue
		}
		out = append(out, j)
	}
	if !found {
		fileMu.Unlock()
		return "", fmt.Errorf("unknown job_id %q", id)
	}
	f.Jobs = out
	err = write(path, f)
	fileMu.Unlock()
	if err != nil {
		return "", err
	}
	slog.Info("schedule.job_removed", "id", id, "path", path)
	notifyScheduleWake()
	return fmt.Sprintf("removed scheduled job %s", id), nil
}

// ListText returns a human-readable list for the tool.
func ListText(cwd string) (string, error) {
	if Disabled() {
		return "scheduled tasks are disabled", nil
	}
	path := Path(cwd)
	fileMu.Lock()
	defer fileMu.Unlock()
	f, err := Read(path)
	if err != nil {
		return "", err
	}
	removed := compactDisabledJobs(f)
	if removed > 0 {
		slog.Info("schedule.compacted_disabled", "removed", removed, "path", path)
	}
	if removed > 0 {
		if err := write(path, f); err != nil {
			return "", err
		}
		notifyScheduleWake()
	}
	if len(f.Jobs) == 0 {
		return "No scheduled jobs.", nil
	}
	var b strings.Builder
	b.WriteString("Scheduled jobs:\n")
	for _, j := range f.Jobs {
		sch := formatSchedule(j.Schedule)
		next := "—"
		if !j.NextRun.IsZero() {
			next = j.NextRun.UTC().Format(time.RFC3339)
		}
		sk := ""
		if j.SessionKey != "" {
			sk = " session_key=" + j.SessionKey
		}
		fmt.Fprintf(&b, "- %s id=%s target=%s%s next=%s — %s\n  schedule: %s\n", j.Name, j.ID, j.TargetSource, sk, next, j.Message, sch)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func formatSchedule(s ScheduleSpec) string {
	switch s.Kind {
	case "at":
		return "once at " + s.AtRFC3339
	case "every":
		return fmt.Sprintf("every %ds", s.EverySeconds)
	case "cron":
		return "cron " + s.CronExpr
	default:
		return s.Kind
	}
}

// FormatScheduledUserText is the user turn text for a fired schedule job (same string is used for the model and transcript).
func FormatScheduledUserText(jobName, jobID string, firedAt time.Time, message string) string {
	fired := firedAt.UTC().Format(time.RFC3339)
	task := formatScheduledTaskLine(jobName, jobID)
	body := strings.TrimSpace(message)
	if body == "" {
		body = "（无正文）"
	}
	return fmt.Sprintf("【定时器触发】\n来源：定时任务\n触发时间（UTC）：%s\n任务：%s\n\n---\n任务正文：\n\n%s", fired, task, body)
}

func formatScheduledTaskLine(name, id string) string {
	name = strings.TrimSpace(name)
	id = strings.TrimSpace(id)
	switch {
	case name != "" && id != "":
		return fmt.Sprintf("%s（id=%s）", name, id)
	case name != "":
		return name
	case id != "":
		return id
	default:
		return "（未命名）"
	}
}

// TurnDelivery is one injected user turn produced by CollectDue.
type TurnDelivery struct {
	Text          string
	CorrelationID string
	SessionKey    string
	UserID        string
	TenantID      string
}

// CollectDue finds jobs for targetSource due at or before `now`, updates their next_run / last_run in the file, and returns deliveries.
// Must be called without holding fileMu; this function locks internally.
func CollectDue(cwd, targetSource string, now time.Time) ([]TurnDelivery, error) {
	if Disabled() {
		return nil, nil
	}
	path := Path(cwd)
	fileMu.Lock()
	defer fileMu.Unlock()
	f, err := Read(path)
	if err != nil {
		return nil, err
	}
	removed := compactDisabledJobs(f)
	changed := removed > 0
	if removed > 0 {
		slog.Info("schedule.compacted_disabled", "removed", removed, "path", path)
	}
	nowUTC := now.UTC()
	var deliveries []TurnDelivery
	var out []Job
	for _, j := range f.Jobs {
		j := j
		if !j.Enabled || j.TargetSource != targetSource {
			out = append(out, j)
			continue
		}
		if j.NextRun.IsZero() || nowUTC.Before(j.NextRun.UTC()) {
			out = append(out, j)
			continue
		}
		dueAt := j.NextRun.UTC()
		firedAt := nowUTC
		t := firedAt
		j.LastRun = &t
		text := FormatScheduledUserText(j.Name, j.ID, firedAt, j.Message)
		corr := fmt.Sprintf("schedule-%s-%d", j.ID, firedAt.UnixNano())
		deliveries = append(deliveries, TurnDelivery{
			Text:          text,
			CorrelationID: corr,
			SessionKey:    j.SessionKey,
			UserID:        j.UserID,
			TenantID:      j.TenantID,
		})
		changed = true

		switch j.Schedule.Kind {
		case "at":
			slog.Info("schedule.job_fired",
				"id", j.ID,
				"name", j.Name,
				"kind", "at",
				"action", "removed_after_fire",
				"due_at", dueAt.Format(time.RFC3339),
				"fired_at", firedAt.Format(time.RFC3339),
				"correlation_id", corr,
				"target", j.TargetSource,
				"session_key", j.SessionKey,
				"message_preview", truncate(j.Message, 200),
				"path", path,
			)
			// one-shot: drop from file so the list does not grow
		case "every":
			next, err := NextRunAfter(j.Schedule, firedAt)
			if err != nil {
				slog.Warn("schedule.advance_failed", "kind", "every", "id", j.ID, "err", err)
			} else {
				j.NextRun = next.UTC()
				out = append(out, j)
				slog.Info("schedule.job_fired",
					"id", j.ID,
					"name", j.Name,
					"kind", "every",
					"action", "rescheduled",
					"due_at", dueAt.Format(time.RFC3339),
					"next_run", j.NextRun.Format(time.RFC3339),
					"every_seconds", j.Schedule.EverySeconds,
					"fired_at", firedAt.Format(time.RFC3339),
					"correlation_id", corr,
					"target", j.TargetSource,
					"session_key", j.SessionKey,
					"message_preview", truncate(j.Message, 200),
					"path", path,
				)
			}
		case "cron":
			next, err := NextRunAfter(j.Schedule, firedAt)
			if err != nil {
				slog.Warn("schedule.advance_failed", "kind", "cron", "id", j.ID, "err", err)
			} else {
				j.NextRun = next.UTC()
				out = append(out, j)
				slog.Info("schedule.job_fired",
					"id", j.ID,
					"name", j.Name,
					"kind", "cron",
					"action", "rescheduled",
					"due_at", dueAt.Format(time.RFC3339),
					"next_run", j.NextRun.Format(time.RFC3339),
					"cron_expr", j.Schedule.CronExpr,
					"fired_at", firedAt.Format(time.RFC3339),
					"correlation_id", corr,
					"target", j.TargetSource,
					"session_key", j.SessionKey,
					"message_preview", truncate(j.Message, 200),
					"path", path,
				)
			}
		default:
			slog.Warn("schedule.drop_unknown_kind", "id", j.ID, "kind", j.Schedule.Kind)
		}
	}
	f.Jobs = out
	if changed {
		if err := write(path, f); err != nil {
			return nil, err
		}
		notifyScheduleWake()
	}
	return deliveries, nil
}
