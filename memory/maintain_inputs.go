package memory

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func maintenanceLogDays() int {
	n := getenvIntMaint("ONCLAW_MAINTENANCE_LOG_DAYS", 3)
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
	return getenvIntMaint("ONCLAW_POST_TURN_MAINTENANCE_MIN_LOG_BYTES", 200)
}

func postTurnMaintenanceMemoryPreviewBytes() int {
	n := getenvIntMaint("ONCLAW_POST_TURN_MAINTENANCE_MEMORY_PREVIEW_BYTES", 4000)
	if n < 1024 {
		n = 1024
	}
	if n > 24_000 {
		n = 24_000
	}
	return n
}

func scheduledMaintainTimeout() time.Duration {
	// Far-field runs are multi-step (read-only tools); be generous so slow APIs rarely hit deadline.
	return maintenanceTimeoutSeconds("ONCLAW_SCHEDULED_MAINTENANCE_TIMEOUT_SEC", 1800)
}

func postTurnMaintainTimeout() time.Duration {
	return maintenanceTimeoutSeconds("ONCLAW_POST_TURN_MAINTENANCE_TIMEOUT_SEC", 60)
}

// scheduledMaintainMaxSteps caps model↔tool rounds for far-field scheduled maintenance (read-only tools).
func scheduledMaintainMaxSteps() int {
	n := getenvIntMaint("ONCLAW_SCHEDULED_MAINTENANCE_MAX_STEPS", 24)
	if n < 2 {
		n = 2
	}
	if n > 64 {
		n = 64
	}
	return n
}

func maintenanceTimeoutSeconds(envKey string, defaultSec int) time.Duration {
	v := strings.TrimSpace(os.Getenv(envKey))
	if v == "" {
		return time.Duration(defaultSec) * time.Second
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return time.Duration(defaultSec) * time.Second
	}
	if n > 3600 {
		n = 3600
	}
	return time.Duration(n) * time.Second
}

func maintenanceMaxTopicFiles() int {
	n := getenvIntMaint("ONCLAW_MAINTENANCE_MAX_TOPIC_FILES", 12)
	if n < 0 {
		n = 0
	}
	if n > 40 {
		n = 40
	}
	return n
}

func maintenanceTopicExcerptBytes() int {
	n := getenvIntMaint("ONCLAW_MAINTENANCE_TOPIC_EXCERPT_BYTES", 2048)
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
