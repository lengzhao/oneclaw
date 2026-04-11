package memory

import (
	"os"
	"time"

	"github.com/lengzhao/oneclaw/rtopts"
)

func maintenanceLogDays() int {
	n := rtopts.Current().MaintenanceLogDays
	if n < 1 {
		n = 1
	}
	if n > 14 {
		n = 14
	}
	return n
}

// postTurnMaintenanceMinLogBytes: minimum UTF-8 size of formatted Current turn snapshot before running near-field maintain.
func postTurnMaintenanceMinLogBytes() int {
	return rtopts.Current().PostTurnMinLogBytes
}

func postTurnMaintenanceMemoryPreviewBytes() int {
	n := rtopts.Current().PostTurnMemoryPreviewBytes
	if n < 1024 {
		n = 1024
	}
	if n > 24_000 {
		n = 24_000
	}
	return n
}

func scheduledMaintainTimeout() time.Duration {
	return rtopts.Current().ScheduledMaintainTimeout
}

func postTurnMaintainTimeout() time.Duration {
	return rtopts.Current().PostTurnMaintainTimeout
}

// scheduledMaintainMaxSteps caps model↔tool rounds for far-field scheduled maintenance (read-only tools).
func scheduledMaintainMaxSteps() int {
	n := rtopts.Current().ScheduledMaintainMaxSteps
	if n < 2 {
		n = 2
	}
	if n > 64 {
		n = 64
	}
	return n
}

func maintenanceMaxTopicFiles() int {
	n := rtopts.Current().MaintenanceMaxTopicFiles
	if n < 0 {
		n = 0
	}
	if n > 40 {
		n = 40
	}
	return n
}

func maintenanceTopicExcerptBytes() int {
	n := rtopts.Current().MaintenanceTopicExcerptBytes
	if n < 256 {
		n = 256
	}
	if n > 16_000 {
		n = 16_000
	}
	return n
}

// countRecentDailyLogBytes sums raw on-disk bytes for a recent calendar-day window of daily logs (no prompt build).
func countRecentDailyLogBytes(autoDir, anchorDate string, days, minBytesPerFile int) int {
	if days < 1 {
		return 0
	}
	t, err := time.ParseInLocation("2006-01-02", anchorDate, time.Local)
	if err != nil {
		return 0
	}
	sum := 0
	for d := 0; d < days; d++ {
		day := t.AddDate(0, 0, -d)
		ds := day.Format("2006-01-02")
		p := DailyLogPath(autoDir, ds)
		data, err := os.ReadFile(p)
		if err != nil || len(data) < minBytesPerFile {
			continue
		}
		sum += len(data)
	}
	return sum
}
