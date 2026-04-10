package memory

import (
	"strconv"
	"strings"
)

// defaultEpisodicAutoMaintainMaxBytes caps the merged same-day "## Auto-maintained" block size
// after dedupe/merge. Oldest bullets are dropped first to stay under the limit.
const defaultEpisodicAutoMaintainMaxBytes = 128 * 1024

// trimEpisodicAutoMaintainSection shortens the digest section by removing bullet lines from the top
// (oldest first, as produced by mergeSameDayAutoMaintainedBlocks). The header line is always kept.
func trimEpisodicAutoMaintainSection(section, digestHeader string, maxBytes int) string {
	if maxBytes <= 0 || len(section) <= maxBytes {
		return section
	}
	headerNorm := strings.TrimSpace(digestHeader)
	lines := strings.Split(section, "\n")
	headerIdx := -1
	for i, l := range lines {
		if strings.TrimSpace(l) == headerNorm {
			headerIdx = i
			break
		}
	}
	if headerIdx < 0 {
		return utf8SafePrefix(section, maxBytes) + "\n…"
	}
	var bullets []string
	for _, l := range lines[headerIdx+1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if bulletLine.MatchString(l) {
			bullets = append(bullets, l)
		}
	}
	dropped := 0
	for len(bullets) > 0 {
		b := rebuildAutoMaintainedSection(headerNorm, bullets)
		if len(b) <= maxBytes {
			return b
		}
		bullets = bullets[1:]
		dropped++
	}
	note := "- (digest trimmed: size cap; dropped " + strconv.Itoa(dropped) + " older bullet(s))"
	b := rebuildAutoMaintainedSection(headerNorm, []string{note})
	if len(b) <= maxBytes {
		return b
	}
	return utf8SafePrefix(section, maxBytes) + "\n…"
}

func rebuildAutoMaintainedSection(headerNorm string, bullets []string) string {
	var b strings.Builder
	b.WriteString(headerNorm)
	b.WriteByte('\n')
	for _, l := range bullets {
		b.WriteString(strings.TrimRight(l, "\r"))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}
