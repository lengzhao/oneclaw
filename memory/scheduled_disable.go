package memory

import "github.com/lengzhao/oneclaw/rtopts"

// ScheduledMaintenanceBackgroundDisabled is true when scheduled / interval background maintenance
// must not run (embedded maintainloop, cmd/maintain interval loop). Does not apply to explicit
// oneclaw -maintain-once or cmd/maintain -once. Config: features.disable_scheduled_maintenance.
func ScheduledMaintenanceBackgroundDisabled() bool {
	return rtopts.Current().DisableScheduledMaintenance
}
