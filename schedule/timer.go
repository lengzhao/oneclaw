package schedule

import "time"

const defaultIdleSleep = time.Hour

var defaultMinTimerSleep = time.Second * time.Duration(MinGranularitySeconds)

// MinTimerSleep is the minimum sleep used with [NextWakeDuration] (coarse wake aligned with [MinGranularitySeconds]).
func MinTimerSleep() time.Duration {
	return defaultMinTimerSleep
}

// IdleTimerSleep is used when no enabled job has a next run in the store.
func IdleTimerSleep() time.Duration {
	return defaultIdleSleep
}

// NextWakeDuration returns how long to sleep until the earliest enabled job's NextRunUnix.
// If ok is false, there is no schedulable job. If due now or overdue, d is 0 and ok is true.
func NextWakeDuration(path string, now time.Time) (d time.Duration, ok bool) {
	f, err := Load(path)
	if err != nil || len(f.Jobs) == 0 {
		return 0, false
	}
	nowU := now.UTC().Unix()
	var minUnix int64
	found := false
	for i := range f.Jobs {
		j := &f.Jobs[i]
		j.Normalize()
		if !j.Enabled || j.NextRunUnix <= 0 {
			continue
		}
		nr := j.NextRunUnix
		if !found || nr < minUnix {
			minUnix = nr
			found = true
		}
	}
	if !found {
		return 0, false
	}
	if minUnix <= nowU {
		return 0, true
	}
	next := time.Unix(minUnix, 0).UTC()
	return next.Sub(now.UTC()), true
}
