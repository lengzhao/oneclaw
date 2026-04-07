package schedule

import (
	"time"

	"github.com/lengzhao/oneclaw/rtopts"
)

// NextWakeDuration returns how long to sleep until the earliest enabled job for targetSource fires.
// If ok is false, there is no next run (no matching jobs or all without NextRun).
// If due immediately or overdue, d is 0 and ok is true.
func NextWakeDuration(cwd, targetSource string, now time.Time) (d time.Duration, ok bool) {
	if Disabled() {
		return 0, false
	}
	path := Path(cwd)
	fileMu.Lock()
	defer fileMu.Unlock()
	f, err := Read(path)
	if err != nil || len(f.Jobs) == 0 {
		return 0, false
	}
	nowUTC := now.UTC()
	var minT time.Time
	found := false
	for _, j := range f.Jobs {
		if !j.Enabled || j.TargetSource != targetSource {
			continue
		}
		if j.NextRun.IsZero() {
			continue
		}
		nr := j.NextRun.UTC()
		if !found || nr.Before(minT) {
			minT = nr
			found = true
		}
	}
	if !found {
		return 0, false
	}
	if !minT.After(nowUTC) {
		return 0, true
	}
	return minT.Sub(nowUTC), true
}

// MinTimerSleep is the minimum sleep used with NextWake (avoids sub-second spin). Config schedule.min_sleep, default 1s.
func MinTimerSleep() time.Duration {
	d := rtopts.Current().ScheduleMinSleep
	if d < 100*time.Millisecond {
		return time.Second
	}
	return d
}

// IdleTimerSleep when no scheduled next run exists for this channel. Config schedule.idle_sleep, default 1h.
func IdleTimerSleep() time.Duration {
	d := rtopts.Current().ScheduleIdleSleep
	if d < time.Second {
		return time.Hour
	}
	return d
}
