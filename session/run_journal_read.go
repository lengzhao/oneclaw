package session

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
		if ev.Phase == "run_complete" {
			return true
		}
	}
	return false
}

// ReadRunJournalText reads runs/<agentType>/runs.jsonl under sessionRoot.
// scope is current_turn or full (default current_turn when non-empty corr).
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
	path := filepath.Join(sr, "runs", at, "runs.jsonl")
	corr := strings.TrimSpace(correlationID)

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("run journal not found at %s", path)
		}
		return "", err
	}
	if sc == "full" || corr == "" {
		return string(b), nil
	}

	const retryPause = 25 * time.Millisecond
	for attempt := 0; attempt < 40; attempt++ {
		out := FilterRunJournalByCorrelation(b, corr)
		if strings.TrimSpace(out) == "" {
			if attempt == 39 {
				return "", fmt.Errorf("no journal lines matched correlation_id %q (try scope full)", corr)
			}
			time.Sleep(retryPause)
			b, err = os.ReadFile(path)
			if err != nil {
				return "", err
			}
			continue
		}
		if journalSnippetHasPhaseComplete(out) {
			return out, nil
		}
		if attempt == 39 {
			return out, nil
		}
		time.Sleep(retryPause)
		b, err = os.ReadFile(path)
		if err != nil {
			return "", err
		}
	}
	return "", fmt.Errorf("run journal read exhausted retries")
}
