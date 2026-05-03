package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// DueFunc receives jobs whose NextRunUnix is in the past; implementations enqueue synthetic inbound turns.
type DueFunc func(ctx context.Context, j Job) error

// Poller reloads the JSON store on each tick and invokes onDue for due jobs, updating next fire times.
type Poller struct {
	Path  string
	Now   func() time.Time
	OnDue DueFunc
}

// NewPoller constructs a poller with RFC3339-ish cron parsing (standard 5-field cron + @every descriptors).
func NewPoller(path string, onDue DueFunc) *Poller {
	return &Poller{
		Path:  path,
		Now:   time.Now,
		OnDue: onDue,
	}
}

// Tick loads the store once, fires due jobs, saves mutations.
func (p *Poller) Tick(ctx context.Context) error {
	if p == nil || p.Path == "" || p.OnDue == nil {
		return nil
	}
	return mutateJobsFile(p.Path, func(f *File) (bool, error) {
		now := p.Now().Unix()
		changed := false
		for i := range f.Jobs {
			j := &f.Jobs[i]
			j.Normalize()
			if !j.Enabled || j.NextRunUnix <= 0 || j.NextRunUnix > now {
				continue
			}

			if j.Kind == KindCron && trimCron(j.CronExpr) == "" {
				slog.Error("schedule.tick", "phase", "validate_cron", "path", p.Path, "job_id", j.ID, "err", "empty cron_expr")
				j.Enabled = false
				j.NextRunUnix = 0
				changed = true
				continue
			}

			if err := p.OnDue(ctx, *j); err != nil {
				slog.Error("schedule.tick", "phase", "on_due", "path", p.Path, "job_id", j.ID, "err", err)
				return false, fmt.Errorf("schedule job %q: %w", j.ID, err)
			}

			changed = true
			switch j.Kind {
			case KindCron:
				from := time.Unix(j.NextRunUnix, 0).UTC()
				next, err := nextCronFire(j.CronExpr, from)
				if err != nil {
					slog.Error("schedule.tick", "phase", "cron_next", "path", p.Path, "job_id", j.ID, "err", err)
					j.Enabled = false
					j.NextRunUnix = 0
					break
				}
				j.NextRunUnix = applyMinFireGap(next.UTC().Unix(), now)
				if j.NextRunUnix <= now {
					j.Enabled = false
				}
			default:
				j.Enabled = false
				j.NextRunUnix = 0
			}
		}
		return changed, nil
	})
}

func nextCronFire(expr string, from time.Time) (time.Time, error) {
	s, err := parseCronSchedule(expr)
	if err != nil {
		return time.Time{}, err
	}
	return s.Next(from), nil
}

func trimCron(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}
