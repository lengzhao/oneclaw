package memory

import (
	"os"
	"strings"
)

// ScheduledMaintenanceBackgroundDisabled is true when scheduled / interval background maintenance
// must not run (embedded maintainloop, cmd/maintain interval loop). Does not apply to explicit
// cmd/maintain -once. Set via ONCLAW_DISABLE_SCHEDULED_MAINTENANCE or YAML features.disable_scheduled_maintenance
// (merged through config.ApplyEnvDefaults).
func ScheduledMaintenanceBackgroundDisabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_SCHEDULED_MAINTENANCE"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}
