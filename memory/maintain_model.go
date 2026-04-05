package memory

import (
	"os"
	"strconv"
	"strings"
)

// ResolveMaintenanceModel picks the API model for a maintenance distill pass.
// Post-turn: ONCLAW_MAINTENANCE_MODEL → mainChatModel.
// Scheduled: ONCLAW_MAINTENANCE_SCHEDULED_MODEL → ONCLAW_MAINTENANCE_MODEL → mainChatModel.
// override is true when a maintenance-specific env was set (non-empty).
func ResolveMaintenanceModel(mainChatModel string, scheduled bool) (model string, override bool) {
	mainChatModel = strings.TrimSpace(mainChatModel)
	if scheduled {
		if v := strings.TrimSpace(os.Getenv("ONCLAW_MAINTENANCE_SCHEDULED_MODEL")); v != "" {
			return v, true
		}
	}
	if v := strings.TrimSpace(os.Getenv("ONCLAW_MAINTENANCE_MODEL")); v != "" {
		return v, true
	}
	return mainChatModel, false
}

// MaintenanceMaxOutputTokens returns ONCLAW_MAINTENANCE_MAX_TOKENS if set, otherwise maxOut.
func MaintenanceMaxOutputTokens(maxOut int64) int64 {
	v := strings.TrimSpace(os.Getenv("ONCLAW_MAINTENANCE_MAX_TOKENS"))
	if v == "" {
		return maxOut
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return maxOut
	}
	return n
}
