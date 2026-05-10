package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"
)

// StartDailyJobAtLocalHour runs fn every calendar day at minute 0 of hour in the host's local timezone.
// Stops when ctx is cancelled. hour must be in [0, 23].
func StartDailyJobAtLocalHour(ctx context.Context, hour int, fn func()) error {
	if hour < 0 || hour > 23 {
		return fmt.Errorf("schedule: hour must be 0-23, got %d", hour)
	}
	spec := fmt.Sprintf("0 %d * * *", hour)
	loc := time.Local
	c := cron.New(cron.WithLocation(loc))
	if _, err := c.AddFunc(spec, fn); err != nil {
		return fmt.Errorf("schedule: cron add %q: %w", spec, err)
	}
	c.Start()
	slog.Info("schedule.daily_job.started", "spec", spec, "tz", loc.String())
	go func() {
		<-ctx.Done()
		stopDone := c.Stop()
		<-stopDone.Done()
		slog.Info("schedule.daily_job.stopped", "spec", spec)
	}()
	return nil
}
