package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultTranscriptTurnLimit is the max transcript lines kept after workflow load_transcript for adk_main replay.
// Each line is one user or assistant turn (pairs count as two lines).
const DefaultTranscriptTurnLimit = 80

// LoadTranscriptTurns reads {agent_type}_transcript.jsonl under sessionRoot (oldest first).
// Call before appending the current user turn so replay excludes this round's user message (see wfexec adk_main).
func LoadTranscriptTurns(sessionRoot, agentType string) ([]TranscriptTurn, error) {
	root := strings.TrimSpace(sessionRoot)
	if root == "" {
		return nil, fmt.Errorf("session: empty session root")
	}
	path := transcriptPath(root, agentType)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var out []TranscriptTurn
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var t TranscriptTurn
		if err := json.Unmarshal(line, &t); err != nil {
			return nil, fmt.Errorf("session: %s line %d: %w", filepath.Base(path), lineNum, err)
		}
		out = append(out, t)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// TrimTranscriptTail keeps at most max turns from the end (most recent). max <= 0 means no trimming.
func TrimTranscriptTail(turns []TranscriptTurn, max int) []TranscriptTurn {
	if max <= 0 || len(turns) <= max {
		return turns
	}
	return append([]TranscriptTurn(nil), turns[len(turns)-max:]...)
}
