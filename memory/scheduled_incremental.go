package memory

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/rtopts"
)

const scheduledMaintainStateFile = "scheduled_maintain_state.json"

// scheduledMaintainStatePath is under layout.DotOrDataRoot() (repo: <cwd>/.oneclaw; IM host: user data root).
func scheduledMaintainStatePath(layout Layout) string {
	return filepath.Join(layout.DotOrDataRoot(), scheduledMaintainStateFile)
}

// migrateScheduledMaintainState moves state from legacy paths under layout.Project/.oneclaw/ or layout.Project/ to DotOrDataRoot().
func migrateScheduledMaintainState(layout Layout) {
	newPath := scheduledMaintainStatePath(layout)
	oldNested := filepath.Join(layout.Project, DotDir, scheduledMaintainStateFile)
	oldFlat := filepath.Join(layout.Project, scheduledMaintainStateFile)
	oldPath := oldNested
	if _, err := os.Stat(oldPath); err != nil {
		oldPath = oldFlat
	}
	if newPath == oldPath {
		return
	}
	if _, err := os.Stat(newPath); err == nil {
		_ = os.Remove(oldPath)
		return
	}
	if _, err := os.Stat(oldPath); err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		slog.Warn("memory.maintain.state_migrate_mkdir", "path", filepath.Dir(newPath), "err", err)
		return
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		b, rerr := os.ReadFile(oldPath)
		if rerr != nil {
			slog.Warn("memory.maintain.state_migrate_read", "path", oldPath, "err", rerr)
			return
		}
		if werr := os.WriteFile(newPath, b, 0o644); werr != nil {
			slog.Warn("memory.maintain.state_migrate_write", "path", newPath, "err", werr)
			return
		}
		_ = os.Remove(oldPath)
	}
	slog.Info("memory.maintain.state_migrated", "from", oldPath, "to", newPath)
	legacyDir := filepath.Dir(oldPath)
	if err := os.Remove(legacyDir); err == nil {
		slog.Debug("memory.maintain.state_legacy_dir_removed", "path", legacyDir)
	}
}

type scheduledMaintainStateJSON struct {
	// HighWaterLogUTC is a legacy line-timestamp cursor (optional).
	HighWaterLogUTC string `json:"high_water_log_utc,omitempty"`
	// LastSuccessWallUTC is wall time after the last successful scheduled far-field pass (tool mode).
	LastSuccessWallUTC string `json:"last_success_wall_utc,omitempty"`
}

func maintenanceIncrementalOverlap() time.Duration {
	return rtopts.Current().MaintenanceIncrementalOverlap
}

func maintenanceIncrementalMaxSpan() time.Duration {
	return rtopts.Current().MaintenanceIncrementalMaxSpan
}

func loadScheduledState(path string) (lastWall *time.Time, lineHigh *time.Time, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var st scheduledMaintainStateJSON
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, nil, err
	}
	if s := strings.TrimSpace(st.LastSuccessWallUTC); s != "" {
		if t, e := time.Parse(time.RFC3339, s); e == nil {
			u := t.UTC()
			lastWall = &u
		}
	}
	if s := strings.TrimSpace(st.HighWaterLogUTC); s != "" {
		if t, e := time.Parse(time.RFC3339, s); e == nil {
			u := t.UTC()
			lineHigh = &u
		}
	}
	return lastWall, lineHigh, nil
}

func saveScheduledLastSuccess(path string, wallUTC time.Time) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var st scheduledMaintainStateJSON
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &st)
	}
	st.LastSuccessWallUTC = wallUTC.UTC().Format(time.RFC3339)
	out, err := json.MarshalIndent(&st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0o644)
}

func persistScheduledMaintainSuccess(statePath string, p distillConfig) {
	if p.pathway != pathwayScheduled || statePath == "" {
		return
	}
	wall := time.Now().UTC()
	if err := saveScheduledLastSuccess(statePath, wall); err != nil {
		slog.Warn("memory.maintain.scheduled_state_write_failed", "path", statePath, "err", err)
		return
	}
	slog.Info("memory.maintain.scheduled_state_updated", "path", statePath, "last_success_wall_utc", wall.Format(time.RFC3339))
}

// incrementalLineMinExclusive returns the lower bound (exclusive) for daily log line timestamps.
func incrementalLineMinExclusive(lastWall, lineHigh *time.Time, interval time.Duration) time.Time {
	nowUTC := time.Now().UTC()
	overlap := maintenanceIncrementalOverlap()
	maxSpan := maintenanceIncrementalMaxSpan()
	floor := nowUTC.Add(-maxSpan)

	var minExclusive time.Time
	if lineHigh != nil && !lineHigh.IsZero() {
		minExclusive = lineHigh.UTC().Add(-overlap)
	} else if lastWall != nil && !lastWall.IsZero() {
		minExclusive = lastWall.UTC().Add(-overlap)
	} else {
		minExclusive = nowUTC.Add(-interval)
	}
	if minExclusive.Before(floor) {
		minExclusive = floor
	}
	return minExclusive
}

func countFilteredDailyLogBytesSince(autoDir string, minExclusive time.Time) int {
	untilUTC := time.Now().UTC()
	startDay := truncateToLocalDate(minExclusive)
	endDay := truncateToLocalDate(untilUTC)
	sum := 0
	for d := endDay; !d.Before(startDay); d = d.AddDate(0, 0, -1) {
		ds := d.Format("2006-01-02")
		p := DailyLogPath(autoDir, ds)
		data, err := os.ReadFile(p)
		if err != nil || len(data) == 0 {
			continue
		}
		f := filterDailyLogBytesAfter(data, minExclusive, untilUTC)
		sum += len(f)
	}
	return sum
}

// parseDailyLogLineTime parses the leading RFC3339 timestamp from a daily log line:
// "- 2006-01-02T15:04:05Z | user: ..."
func parseDailyLogLineTime(line string) (time.Time, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "- ") {
		return time.Time{}, false
	}
	rest := strings.TrimPrefix(line, "- ")
	idx := strings.Index(rest, " |")
	if idx < 0 {
		return time.Time{}, false
	}
	tsStr := strings.TrimSpace(rest[:idx])
	t, err := time.Parse(time.RFC3339, tsStr)
	if err != nil {
		return time.Time{}, false
	}
	return t.UTC(), true
}

func filterDailyLogBytesAfter(data []byte, minExclusive time.Time, untilUTC time.Time) []byte {
	if len(data) == 0 {
		return nil
	}
	var b strings.Builder
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		ts, ok := parseDailyLogLineTime(line)
		if !ok {
			continue
		}
		if !ts.After(minExclusive) {
			continue
		}
		if ts.After(untilUTC) {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return []byte(strings.TrimSuffix(b.String(), "\n"))
}

func truncateToLocalDate(t time.Time) time.Time {
	t = t.In(time.Local)
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.Local)
}
