package session

import (
	"path/filepath"
	"strings"
)

// TurnJournalKey names runs/<agent>/<key>.jsonl files.
// Root turns use correlationID only; delegated sub-agent turns use correlationID__<basename(sessionRoot)>
// when sessionRoot points at sessions/.../subs/<sub_run_id>/.
func TurnJournalKey(correlationID string, delegationDepth int, sessionRoot string) string {
	corr := strings.TrimSpace(correlationID)
	if corr == "" {
		return ""
	}
	if delegationDepth > 0 {
		base := filepath.Base(strings.TrimSpace(sessionRoot))
		if base != "" && base != "." && base != "/" {
			return corr + "__" + base
		}
	}
	return corr
}
