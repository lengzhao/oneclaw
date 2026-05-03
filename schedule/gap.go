package schedule

// MinSecondsBetweenJobFires enforces at least this many seconds between consecutive fires of the same cron job (wall-clock from last tick).
const MinSecondsBetweenJobFires int64 = 60

func applyMinFireGap(nextUnix, firedAtUnix int64) int64 {
	floor := firedAtUnix + MinSecondsBetweenJobFires
	if nextUnix < floor {
		return floor
	}
	return nextUnix
}
