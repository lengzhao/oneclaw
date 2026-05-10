package memory

import "github.com/lengzhao/oneclaw/rtopts"

// ScheduledMaintenanceBackgroundDisabled is true when features.disable_scheduled_maintenance is set.
// Disables the resident process embedded daily RunScheduledMaintain cron (see cmd/oneclaw).
// External schedulers calling RunScheduledMaintain should also respect this flag when appropriate.
// Does not apply to explicit oneclaw -maintain-once.
func ScheduledMaintenanceBackgroundDisabled() bool {
	return rtopts.Current().DisableScheduledMaintenance
}
