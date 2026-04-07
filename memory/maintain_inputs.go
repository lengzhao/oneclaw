package memory

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// collectRecentDailyLogs concatenates up to `days` daily logs ending at `anchorDate` (YYYY-MM-DD),
// newest first in the output. Each file is capped to maxPerFile runes-bytes via utf8SafePrefix;
// total output is capped to maxTotal bytes.
func collectRecentDailyLogs(autoDir, anchorDate string, days, minBytesPerFile, maxPerFile, maxTotal int) (combined string, includedBytes int) {
	if days < 1 || maxTotal <= 0 {
		return "", 0
	}
	t, err := time.ParseInLocation("2006-01-02", anchorDate, time.Local)
	if err != nil {
		return "", 0
	}
	var b strings.Builder
	for d := 0; d < days; d++ {
		day := t.AddDate(0, 0, -d)
		ds := day.Format("2006-01-02")
		p := DailyLogPath(autoDir, ds)
		data, err := os.ReadFile(p)
		if err != nil || len(data) < minBytesPerFile {
			continue
		}
		includedBytes += len(data)
		excerpt := string(data)
		if len(excerpt) > maxPerFile {
			excerpt = strings.TrimRight(utf8SafePrefix(excerpt, maxPerFile), "\n") + "\n\n…"
		}
		if b.Len() > 0 {
			b.WriteString("\n---\n")
		}
		b.WriteString("### Daily log ")
		b.WriteString(ds)
		b.WriteString("\n\n")
		b.WriteString(excerpt)
		if b.Len() >= maxTotal {
			s := b.String()
			return strings.TrimRight(utf8SafePrefix(s, maxTotal), "\n") + "\n\n…", includedBytes
		}
	}
	return b.String(), includedBytes
}

// countRecentDailyLogBytes sums raw on-disk bytes for the same calendar-day window as collectRecentDailyLogs (no prompt build).
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

// collectProjectTopicExcerpts lists `*.md` directly under project memory (excluding MEMORY.md)
// and returns a bounded markdown block for the maintenance prompt.
func collectProjectTopicExcerpts(projectDir string, maxFiles, excerptBytes, maxTotal int) string {
	if maxFiles <= 0 || maxTotal <= 0 {
		return ""
	}
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		n := e.Name()
		if !strings.EqualFold(filepath.Ext(n), ".md") {
			continue
		}
		if strings.EqualFold(n, entrypointName) {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)
	if len(names) > maxFiles {
		names = names[:maxFiles]
	}
	var b strings.Builder
	for _, name := range names {
		p := filepath.Join(projectDir, name)
		data, err := os.ReadFile(p)
		if err != nil || len(data) == 0 {
			continue
		}
		body := string(data)
		if len(body) > excerptBytes {
			body = strings.TrimRight(utf8SafePrefix(body, excerptBytes), "\n") + "\n\n…"
		}
		block := "### topic: " + name + "\n\n```\n" + body + "\n```\n"
		if b.Len()+len(block) >= maxTotal {
			if b.Len() == 0 {
				return strings.TrimRight(utf8SafePrefix(block, maxTotal), "\n") + "\n\n…"
			}
			break
		}
		b.WriteString(block)
	}
	return strings.TrimSpace(b.String())
}
