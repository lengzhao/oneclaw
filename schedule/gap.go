package schedule

const (
	// MinGranularitySeconds is the minimum scheduling resolution: poller sleep floor, spacing between
	// cron fires, and minimum at_seconds / every_seconds / RFC3339 lead time for the cron tool.
	MinGranularitySeconds int64 = 10

	// MinSecondsBetweenJobFires enforces at least this many seconds between consecutive fires of the same cron job (wall-clock from last tick).
	MinSecondsBetweenJobFires = MinGranularitySeconds
)

func applyMinFireGap(nextUnix, firedAtUnix int64) int64 {
	floor := firedAtUnix + MinSecondsBetweenJobFires
	if nextUnix < floor {
		return floor
	}
	return nextUnix
}
