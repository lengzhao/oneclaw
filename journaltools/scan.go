// Package journaltools summarizes host turn run journals (JSONL) for workflow gates and tooling.
package journaltools

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

// ScanOpts configures optional journal aggregation behavior.
type ScanOpts struct {
	// MatchTools lists tool_name values (exact match after TrimSpace) to track as NamedToolsPresent / NamedToolsCounts.
	MatchTools []string
}

// Metrics summarizes one run journal JSONL file for gates and observability.
type Metrics struct {
	DistinctToolCalls int  `json:"distinct_tool_calls"`
	SkillTreeToolUsed bool `json:"skill_tree_tool_used"`
	// ToolCallEvents counts every JSON line with phase tool_call (including duplicate tool_call_id rows).
	ToolCallEvents int `json:"tool_call_events"`
	// LineCount counts successfully unmarshaled RunEvent lines (non-empty JSON objects).
	LineCount int `json:"line_count"`

	DurationSeconds float64 `json:"duration_seconds"`
	FirstTs         string  `json:"first_ts,omitempty"`
	LastTs          string  `json:"last_ts,omitempty"`
	HasRunStart     bool    `json:"has_run_start"`
	HasRunComplete  bool    `json:"has_run_complete"`

	// ToolNameCounts counts tool_call rows by tool_name (every row, not deduped by tool_call_id).
	ToolNameCounts map[string]int `json:"tool_name_counts"`

	NamedToolsPresent map[string]bool `json:"named_tools_present,omitempty"`
	NamedToolsCounts  map[string]int  `json:"named_tools_counts,omitempty"`
}

// ScanJournalFile reads one journal JSONL file (may be missing).
func ScanJournalFile(journalPath, userDataRoot string, opts *ScanOpts) (Metrics, error) {
	var out Metrics
	out.ToolNameCounts = map[string]int{}
	path := strings.TrimSpace(journalPath)
	if path == "" {
		return out, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return out, err
	}
	defer f.Close()

	udr := strings.TrimSpace(userDataRoot)
	var matchNorm []string
	if opts != nil {
		for _, s := range opts.MatchTools {
			s = strings.TrimSpace(s)
			if s != "" {
				matchNorm = append(matchNorm, s)
			}
		}
	}
	if len(matchNorm) > 0 {
		out.NamedToolsPresent = map[string]bool{}
		out.NamedToolsCounts = map[string]int{}
		for _, s := range matchNorm {
			out.NamedToolsPresent[s] = false
			out.NamedToolsCounts[s] = 0
		}
	}

	seen := map[string]bool{}
	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	var minTs, maxTs time.Time
	haveTs := false

	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var ev session.RunEvent
		if json.Unmarshal(line, &ev) != nil {
			continue
		}
		out.LineCount++

		if !ev.Ts.IsZero() {
			if !haveTs || ev.Ts.Before(minTs) {
				minTs = ev.Ts
			}
			if !haveTs || ev.Ts.After(maxTs) {
				maxTs = ev.Ts
			}
			haveTs = true
		}

		switch strings.TrimSpace(strings.ToLower(ev.Phase)) {
		case "run_start":
			out.HasRunStart = true
		case "run_complete":
			out.HasRunComplete = true
		case "tool_call":
			out.ToolCallEvents++
			if ev.Detail == nil {
				continue
			}
			id, _ := ev.Detail["tool_call_id"].(string)
			id = strings.TrimSpace(id)
			if id != "" {
				if seen[id] {
					continue
				}
				seen[id] = true
			}
			out.DistinctToolCalls++

			tname, _ := ev.Detail["tool_name"].(string)
			tname = strings.TrimSpace(tname)
			if tname != "" {
				out.ToolNameCounts[tname]++
			}

			args, _ := ev.Detail["arguments"].(string)
			if toolCallTargetsSkills(udr, tname, args) {
				out.SkillTreeToolUsed = true
			}
		}
	}
	if err := sc.Err(); err != nil {
		return out, err
	}

	if haveTs && !minTs.IsZero() && !maxTs.IsZero() {
		out.DurationSeconds = maxTs.Sub(minTs).Seconds()
		out.FirstTs = minTs.UTC().Format(time.RFC3339Nano)
		out.LastTs = maxTs.UTC().Format(time.RFC3339Nano)
	}

	for _, want := range matchNorm {
		c := out.ToolNameCounts[want]
		out.NamedToolsCounts[want] = c
		out.NamedToolsPresent[want] = c > 0
	}

	return out, nil
}

func toolCallTargetsSkills(userDataRoot, toolName, argumentsJSON string) bool {
	switch strings.TrimSpace(toolName) {
	case builtin.NameReadSkill:
		return true
	case builtin.NameReadFile, builtin.NameWriteFile:
		return pathTargetsUserSkills(userDataRoot, jsonToolPath(argumentsJSON))
	default:
		return false
	}
}

func jsonToolPath(argumentsJSON string) string {
	raw := strings.TrimSpace(argumentsJSON)
	if raw == "" {
		return ""
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return ""
	}
	rawPath, ok := m["path"]
	if !ok || len(rawPath) == 0 {
		return ""
	}
	var path string
	if err := json.Unmarshal(rawPath, &path); err != nil {
		return ""
	}
	return path
}

func pathTargetsUserSkills(userDataRoot, path string) bool {
	p := strings.TrimSpace(path)
	if p == "" {
		return false
	}
	norm := filepath.ToSlash(strings.ToLower(p))
	norm = strings.TrimPrefix(norm, "./")
	if strings.HasPrefix(norm, "skills/") {
		return true
	}
	udr := strings.TrimSpace(userDataRoot)
	if udr == "" || !filepath.IsAbs(p) {
		return false
	}
	skRoot := filepath.Clean(filepath.Join(udr, "skills"))
	abs := filepath.Clean(p)
	rel, err := filepath.Rel(skRoot, abs)
	if err != nil {
		return false
	}
	if rel == "." {
		return false
	}
	return !strings.HasPrefix(rel, "..")
}
