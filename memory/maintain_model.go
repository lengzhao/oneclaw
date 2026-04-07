package memory

import (
	"strings"

	"github.com/lengzhao/oneclaw/rtopts"
)

// ResolveMaintenanceModel picks the API model for a maintenance distill pass from config (maintain.model / scheduled_model).
func ResolveMaintenanceModel(mainChatModel string, scheduled bool) (model string, override bool) {
	mainChatModel = strings.TrimSpace(mainChatModel)
	rt := rtopts.Current()
	if scheduled {
		if v := strings.TrimSpace(rt.MaintenanceScheduledModel); v != "" {
			return v, true
		}
	}
	if v := strings.TrimSpace(rt.MaintenanceModel); v != "" {
		return v, true
	}
	return mainChatModel, false
}

// MaintenanceMaxOutputTokens returns maintain.max_tokens from config when set, otherwise maxOut.
func MaintenanceMaxOutputTokens(maxOut int64) int64 {
	if t := rtopts.Current().MaintenanceMaxTokens; t > 0 {
		return t
	}
	return maxOut
}

// maintenanceEffectiveMaxTokens applies post_turn.max_tokens for post-turn when set, then MaintenanceMaxOutputTokens.
func maintenanceEffectiveMaxTokens(maxOut int64, postTurn bool) int64 {
	if postTurn {
		if t := rtopts.Current().PostTurnMaxTokens; t > 0 {
			return t
		}
	}
	return MaintenanceMaxOutputTokens(maxOut)
}
