package session

import (
	"path/filepath"
	"regexp"
	"strings"
)

const legacyTranscriptFile = "transcript.jsonl"

var nonTranscriptNameRune = regexp.MustCompile(`[^a-z0-9._-]+`)

func transcriptFileName(agentType string) string {
	s := strings.ToLower(strings.TrimSpace(agentType))
	if s == "" {
		return legacyTranscriptFile
	}
	s = strings.ReplaceAll(s, " ", "-")
	s = nonTranscriptNameRune.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-._")
	if s == "" {
		return legacyTranscriptFile
	}
	return s + "_transcript.jsonl"
}

func transcriptPath(sessionRoot, agentType string) string {
	return filepath.Join(sessionRoot, transcriptFileName(agentType))
}
