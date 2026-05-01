package instructions

import (
	"fmt"
	"strings"
)

// Entrypoint line/byte caps align with claude-code memdir.ts.
const (
	MaxEntrypointLines = 200
	MaxEntrypointBytes = 25_000
)

// EntrypointTruncation is the result of truncating a markdown entry file.
type EntrypointTruncation struct {
	Content          string
	LineCount        int
	ByteCount        int
	WasLineTruncated bool
	WasByteTruncated bool
}

// TruncateEntrypointContent applies line cap then byte cap (at last newline before cap).
// displayName is used in the WARNING line (e.g. "MEMORY.md").
func TruncateEntrypointContent(raw, displayName string) EntrypointTruncation {
	trimmed := strings.TrimSpace(raw)
	lines := splitLines(trimmed)
	lineCount := len(lines)
	byteCount := len(trimmed)

	wasLine := lineCount > MaxEntrypointLines
	wasByte := byteCount > MaxEntrypointBytes
	if !wasLine && !wasByte {
		return EntrypointTruncation{
			Content:          trimmed,
			LineCount:        lineCount,
			ByteCount:        byteCount,
			WasLineTruncated: false,
			WasByteTruncated: false,
		}
	}

	truncated := trimmed
	if wasLine {
		truncated = joinLines(lines[:MaxEntrypointLines])
	}
	if len(truncated) > MaxEntrypointBytes {
		cut := lastIndexNewlineBefore(truncated, MaxEntrypointBytes)
		if cut > 0 {
			truncated = truncated[:cut]
		} else {
			truncated = truncated[:MaxEntrypointBytes]
		}
	}

	var reason string
	switch {
	case wasByte && !wasLine:
		reason = fmt.Sprintf("%d bytes (limit: %d) — index entries are too long", byteCount, MaxEntrypointBytes)
	case wasLine && !wasByte:
		reason = fmt.Sprintf("%d lines (limit: %d)", lineCount, MaxEntrypointLines)
	default:
		reason = fmt.Sprintf("%d lines and %d bytes", lineCount, byteCount)
	}

	warning := fmt.Sprintf(
		"\n\n> WARNING: %s is %s. Only part of it was loaded. Keep index entries to one line under ~200 chars; move detail into topic files.",
		displayName,
		reason,
	)
	return EntrypointTruncation{
		Content:          truncated + warning,
		LineCount:        lineCount,
		ByteCount:        byteCount,
		WasLineTruncated: wasLine,
		WasByteTruncated: wasByte,
	}
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	n := len(lines) - 1
	for _, l := range lines {
		n += len(l)
	}
	b := make([]byte, 0, n)
	for i, l := range lines {
		b = append(b, l...)
		if i < len(lines)-1 {
			b = append(b, '\n')
		}
	}
	return string(b)
}

func lastIndexNewlineBefore(s string, max int) int {
	if max > len(s) {
		max = len(s)
	}
	i := max - 1
	for i >= 0 {
		if s[i] == '\n' {
			return i
		}
		i--
	}
	return -1
}
