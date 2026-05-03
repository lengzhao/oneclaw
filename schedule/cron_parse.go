package schedule

import (
	"fmt"

	"github.com/robfig/cron/v3"
)

// cronStdParser matches [Poller] scheduling (minute fields + @every descriptors).
var cronStdParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

func parseCronSchedule(expr string) (cron.Schedule, error) {
	expr = trimCron(expr)
	if expr == "" {
		return nil, fmt.Errorf("empty cron expression")
	}
	return cronStdParser.Parse(expr)
}
