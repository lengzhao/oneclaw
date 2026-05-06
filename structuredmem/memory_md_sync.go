package structuredmem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lzmem "github.com/lengzhao/memory"

	memcore "github.com/lengzhao/oneclaw/memory"
)

const (
	autoSectionTitle = "## Structured memory (auto)"
	autoStartMarker  = "<!-- structuredmem:auto:start -->"
	autoEndMarker    = "<!-- structuredmem:auto:end -->"
	maxAutoLines     = 8
)

// memoryMDPromoteMinConfidence is the minimum model-reported confidence for syncing a profile row into MEMORY.md.
// Extract already applies MinConfidence (default 0.7); this threshold is slightly higher to keep the injected block small.
const memoryMDPromoteMinConfidence = 0.80

// SyncMemoryMDFromExtract appends high-confidence profile memories from the extract result into MEMORY.md.
// Promotion uses only namespace + confidence from the model — no keyword heuristics.
func SyncMemoryMDFromExtract(instructionRoot string, result *lzmem.ExtractResult) error {
	root := strings.TrimSpace(instructionRoot)
	if root == "" || result == nil || len(result.Memories) == 0 {
		return nil
	}
	newLines := promotedMemoryLines(result)
	if len(newLines) == 0 {
		return nil
	}
	path := filepath.Join(root, "MEMORY.md")
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	existing := string(raw)
	merged := mergeAutoLines(existing, newLines)
	if len(merged) == 0 {
		return nil
	}
	candidate := renderWithAutoSection(existing, merged)
	for len([]byte(candidate)) > memcore.MEMORYMDMaxBytes && len(merged) > 0 {
		merged = merged[:len(merged)-1]
		candidate = renderWithAutoSection(existing, merged)
	}
	if len([]byte(candidate)) > memcore.MEMORYMDMaxBytes {
		return nil
	}
	if candidate == existing {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(candidate), 0o644)
}

func promotedMemoryLines(result *lzmem.ExtractResult) []string {
	if result == nil {
		return nil
	}
	lines := make([]string, 0, len(result.Memories))
	seen := make(map[string]struct{}, len(result.Memories))
	for _, m := range result.Memories {
		if !shouldPromoteToMemoryMD(m) {
			continue
		}
		text := strings.TrimSpace(m.Summary)
		if text == "" {
			text = strings.TrimSpace(m.Title)
		}
		if text == "" {
			text = strings.TrimSpace(m.Content)
		}
		text = normalizeInlineText(text)
		if text == "" {
			continue
		}
		line := fmt.Sprintf("- [%s] %s", strings.TrimSpace(string(m.Namespace)), text)
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		lines = append(lines, line)
	}
	return lines
}

func shouldPromoteToMemoryMD(m lzmem.ExtractedMemory) bool {
	if !strings.EqualFold(strings.TrimSpace(string(m.Namespace)), string(lzmem.NamespaceProfile)) {
		return false
	}
	return m.Confidence >= memoryMDPromoteMinConfidence
}

func normalizeInlineText(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func mergeAutoLines(existing string, newLines []string) []string {
	current := extractAutoLines(existing)
	out := make([]string, 0, len(newLines)+len(current))
	seen := map[string]struct{}{}
	push := func(line string) {
		line = normalizeInlineText(line)
		if line == "" {
			return
		}
		if _, ok := seen[line]; ok {
			return
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	for _, line := range newLines {
		push(line)
	}
	for _, line := range current {
		push(line)
	}
	if len(out) > maxAutoLines {
		out = out[:maxAutoLines]
	}
	return out
}

func extractAutoLines(content string) []string {
	start := strings.Index(content, autoStartMarker)
	end := strings.Index(content, autoEndMarker)
	if start < 0 || end < 0 || end <= start {
		return nil
	}
	body := content[start+len(autoStartMarker) : end]
	parts := strings.Split(body, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if strings.HasPrefix(p, "- ") {
			out = append(out, p)
		}
	}
	return out
}

func renderWithAutoSection(existing string, lines []string) string {
	section := autoSectionTitle + "\n\n" + autoStartMarker + "\n" + strings.Join(lines, "\n") + "\n" + autoEndMarker + "\n"
	start := strings.Index(existing, autoStartMarker)
	end := strings.Index(existing, autoEndMarker)
	if start >= 0 && end >= 0 && end > start {
		blockStart := strings.LastIndex(existing[:start], autoSectionTitle)
		if blockStart < 0 {
			blockStart = start
		}
		blockEnd := end + len(autoEndMarker)
		if blockEnd < len(existing) && existing[blockEnd] == '\n' {
			blockEnd++
		}
		out := strings.TrimRight(existing[:blockStart], "\n")
		if out != "" {
			out += "\n\n"
		}
		out += section
		tail := strings.TrimLeft(existing[blockEnd:], "\n")
		if tail != "" {
			out += "\n" + tail
		}
		return strings.TrimRight(out, "\n") + "\n"
	}
	base := strings.TrimRight(existing, "\n")
	if base == "" {
		return section
	}
	return base + "\n\n" + section
}
