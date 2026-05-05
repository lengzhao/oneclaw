package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// JournalLineMatchesCorrelation reports whether a JSONL line's detail.correlation_id equals wantCorr.
func JournalLineMatchesCorrelation(line []byte, wantCorr string) bool {
	line = bytes.TrimSpace(line)
	if len(line) == 0 {
		return false
	}
	var ev struct {
		Detail map[string]any `json:"detail"`
	}
	if err := json.Unmarshal(line, &ev); err != nil {
		return false
	}
	if ev.Detail == nil {
		return false
	}
	raw, ok := ev.Detail["correlation_id"]
	if !ok || raw == nil {
		return false
	}
	got := strings.TrimSpace(fmt.Sprint(raw))
	return got != "" && got == wantCorr
}

// FilterRunJournalByCorrelation keeps JSONL lines whose detail.correlation_id matches wantCorr.
// Empty wantCorr returns the full content unchanged.
func FilterRunJournalByCorrelation(content []byte, wantCorr string) string {
	if strings.TrimSpace(wantCorr) == "" {
		return string(content)
	}
	var out strings.Builder
	for _, line := range bytes.Split(content, []byte{'\n'}) {
		if JournalLineMatchesCorrelation(line, wantCorr) {
			out.Write(line)
			out.WriteByte('\n')
		}
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func journalSnippetHasPhaseComplete(snippet string) bool {
	for _, line := range strings.Split(snippet, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev struct {
			Phase string `json:"phase"`
		}
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		switch ev.Phase {
		case "run_complete", "sub_agent_complete":
			return true
		}
	}
	return false
}

type journalFileInfo struct {
	path string
	mod  time.Time
	name string
}

func readMergedTurnJournals(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("run journal directory not found at %s", dir)
		}
		return "", err
	}
	var files []journalFileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".jsonl") {
			continue
		}
		if name == "runs.jsonl" {
			continue
		}
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err != nil || !st.Mode().IsRegular() {
			continue
		}
		files = append(files, journalFileInfo{path: p, mod: st.ModTime(), name: name})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].mod.Equal(files[j].mod) {
			return files[i].name < files[j].name
		}
		return files[i].mod.Before(files[j].mod)
	})
	var out strings.Builder
	for _, f := range files {
		b, err := os.ReadFile(f.path)
		if err != nil {
			return "", err
		}
		s := strings.TrimSpace(string(b))
		if s == "" {
			continue
		}
		if out.Len() > 0 {
			out.WriteByte('\n')
		}
		out.WriteString(s)
		out.WriteByte('\n')
	}
	if out.Len() == 0 {
		return "", fmt.Errorf("no per-turn run journals under %s", dir)
	}
	return strings.TrimRight(out.String(), "\n") + "\n", nil
}

// ReadRunJournalText reads per-turn JSONL under sessions/<id>/runs/<agent_type>/<key>.jsonl.
// scope full merges all *.jsonl files in that directory (excluding legacy runs.jsonl), ordered by modification time.
// scope current_turn with correlation_id reads exactly that turn file; optional retry until run_complete / sub_agent_complete.
func ReadRunJournalText(sessionRoot, agentType, correlationID, scope string) (string, error) {
	sr := strings.TrimSpace(sessionRoot)
	at := strings.TrimSpace(agentType)
	if sr == "" {
		return "", fmt.Errorf("session root required")
	}
	if at == "" {
		return "", fmt.Errorf("agent type required")
	}
	sc := strings.ToLower(strings.TrimSpace(scope))
	if sc == "" {
		sc = "current_turn"
	}
	if sc != "current_turn" && sc != "full" {
		return "", fmt.Errorf("scope must be current_turn or full")
	}
	dir := filepath.Join(sr, "runs", at)
	corr := strings.TrimSpace(correlationID)

	if sc == "full" || corr == "" {
		return readMergedTurnJournals(dir)
	}

	turnPath := TurnRunJournalPath(sr, at, corr)
	const retryPause = 25 * time.Millisecond
	for attempt := 0; attempt < 40; attempt++ {
		b, err := os.ReadFile(turnPath)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		var content string
		if err == nil {
			content = string(b)
		}
		trimmed := strings.TrimSpace(content)
		if trimmed == "" {
			if attempt == 39 {
				return "", fmt.Errorf("run journal turn file empty or missing: %s", turnPath)
			}
			time.Sleep(retryPause)
			continue
		}
		if journalSnippetHasPhaseComplete(trimmed) || attempt == 39 {
			return strings.TrimRight(trimmed, "\n"), nil
		}
		time.Sleep(retryPause)
	}
	return "", fmt.Errorf("run journal read exhausted retries")
}
