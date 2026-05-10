package memory

import "github.com/lengzhao/oneclaw/rtopts"

// ScheduledMaintenanceBackgroundDisabled is true when features.disable_scheduled_maintenance is set.
// oneclaw no longer embeds a maintenance goroutine; external schedulers calling RunScheduledMaintain
// should respect this flag. Does not apply to explicit oneclaw -maintain-once.
func ScheduledMaintenanceBackgroundDisabled() bool {
	return rtopts.Current().DisableScheduledMaintenance
}
