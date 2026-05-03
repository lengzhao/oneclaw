// Package skillsusage appends skill-use events as JSONL under UserDataRoot/skills/_usage.jsonl
// for digest hot ranking (append-only; no aggregate index file).
package skillsusage

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LogFileName is written under UserDataRoot/skills/ (one JSON object per line).
const LogFileName = "_usage.jsonl"

// Event is a single append-only skill usage record.
type Event struct {
	TimeUnix int64  `json:"ts"`
	SkillID  string `json:"skill_id"`
	Action   string `json:"action,omitempty"`
}

// Record appends one usage line to skills/<LogFileName>. Empty or "_" skill ids are ignored.
func Record(skillsRoot, skillID, action string) error {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" || strings.HasPrefix(skillID, "_") {
		return nil
	}
	skillsRoot = filepath.Clean(strings.TrimSpace(skillsRoot))
	if skillsRoot == "" {
		return fmt.Errorf("skillsusage: empty skills root")
	}
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		return err
	}
	path := filepath.Join(skillsRoot, LogFileName)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	ev := Event{TimeUnix: time.Now().Unix(), SkillID: skillID, Action: strings.TrimSpace(action)}
	enc := json.NewEncoder(f)
	enc.SetEscapeHTML(false)
	return enc.Encode(ev)
}

// Aggregate scans the JSONL log and returns per-skill counts and last-used unix time.
func Aggregate(skillsRoot string) (counts map[string]int64, lastUsed map[string]int64, err error) {
	counts = map[string]int64{}
	lastUsed = map[string]int64{}
	skillsRoot = filepath.Clean(strings.TrimSpace(skillsRoot))
	if skillsRoot == "" {
		return nil, nil, fmt.Errorf("skillsusage: empty skills root")
	}
	path := filepath.Join(skillsRoot, LogFileName)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return counts, lastUsed, nil
		}
		return nil, nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev Event
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		id := strings.TrimSpace(ev.SkillID)
		if id == "" || strings.HasPrefix(id, "_") {
			continue
		}
		counts[id]++
		if ev.TimeUnix > lastUsed[id] {
			lastUsed[id] = ev.TimeUnix
		}
	}
	return counts, lastUsed, sc.Err()
}

// RankSkillIDs returns allIDs sorted by count desc, last_used desc, id asc (unknown ids get zero counts).
func RankSkillIDs(counts, lastUsed map[string]int64, allIDs []string) []string {
	if len(allIDs) == 0 {
		return nil
	}
	if counts == nil {
		counts = map[string]int64{}
	}
	if lastUsed == nil {
		lastUsed = map[string]int64{}
	}
	seen := make(map[string]bool)
	var rs []rankedSkill
	for _, id := range allIDs {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		rs = append(rs, rankedSkill{id: id, count: counts[id], last: lastUsed[id]})
	}
	sort.Slice(rs, func(i, j int) bool {
		if rs[i].count != rs[j].count {
			return rs[i].count > rs[j].count
		}
		if rs[i].last != rs[j].last {
			return rs[i].last > rs[j].last
		}
		return rs[i].id < rs[j].id
	})
	out := make([]string, len(rs))
	for i := range rs {
		out[i] = rs[i].id
	}
	return out
}

type rankedSkill struct {
	id    string
	count int64
	last  int64
}
