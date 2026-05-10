package memory

import (
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/rtopts"
)

// Audit sources for AppendMemoryAudit when recording maintenance-related writes (e.g. agent_memory.sqlite).
const (
	AuditSourcePostTurnMaintain  = "post_turn_maintain"
	AuditSourceScheduledMaintain = "scheduled_maintain"
)

func autoMaintenanceEnabled() bool {
	return !rtopts.Current().DisableAutoMaintenance
}

func maintenanceMinLogBytes() int {
	return rtopts.Current().MaintenanceMinLogBytes
}

func maintenanceMaxLogRead() int {
	return rtopts.Current().MaintenanceMaxLogRead
}

func maintenanceMaxCombinedLogBytes() int {
	return rtopts.Current().MaintenanceMaxCombinedLogBytes
}

func utf8SafePrefix(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for !utf8.ValidString(s) {
		if len(s) == 0 {
			return ""
		}
		s = s[:len(s)-1]
	}
	return s
}
