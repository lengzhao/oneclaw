package memory

import "strings"

// digestHeaderForDate is the exact H2 line for an auto-maintained digest on dateStr (YYYY-MM-DD).
func digestHeaderForDate(dateStr string) string {
	return "## Auto-maintained (" + dateStr + ")"
}

// findSameDayAutoMaintainedSpan returns byte offsets [start, end) covering today's digest block.
// Duplicate same-day headers (legacy duplicates) are included in the span until a different ## section.
func findSameDayAutoMaintainedSpan(md, dateStr string) (start, end int, ok bool) {
	header := digestHeaderForDate(dateStr)
	idx := strings.Index(md, header)
	if idx < 0 {
		return 0, 0, false
	}
	start = idx
	pos := idx + len(header)
	for pos < len(md) && (md[pos] == '\n' || md[pos] == '\r') {
		pos++
	}
	headerNorm := strings.TrimSpace(header)
	for pos < len(md) {
		lineStart := pos
		var line string
		if nl := strings.IndexByte(md[pos:], '\n'); nl >= 0 {
			lineEnd := pos + nl
			line = md[lineStart:lineEnd]
			pos = lineEnd + 1
		} else {
			line = md[lineStart:]
			pos = len(md)
		}
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "## ") {
			if t == headerNorm {
				for pos < len(md) && (md[pos] == '\n' || md[pos] == '\r') {
					pos++
				}
				continue
			}
			return start, lineStart, true
		}
	}
	return start, len(md), true
}

// mergeSameDayAutoMaintainedBlocks merges existingToday's block with model output newSection (same header).
// olderCorpus is MEMORY.md with today's span removed; its bullets seed deduplication.
// Precondition: newSection is non-empty of durable bullets (caller checked maintenanceSectionOnlyNoDurable).
func mergeSameDayAutoMaintainedBlocks(existingTodayBlock, newSection, digestHeader, olderCorpus string) string {
	existingTodayBlock = strings.TrimSpace(existingTodayBlock)
	newSection = strings.TrimSpace(newSection)
	if existingTodayBlock == "" {
		return newSection
	}
	seen := bulletKeysFromMarkdown(olderCorpus)
	var lines []string
	addBullet := func(raw string) {
		raw = strings.TrimRight(raw, "\r")
		m := bulletLine.FindStringSubmatch(raw)
		if m == nil {
			return
		}
		k := normalizeBulletKey(m[1])
		if k == "" {
			return
		}
		if _, dup := seen[k]; dup {
			return
		}
		seen[k] = struct{}{}
		lines = append(lines, strings.TrimSpace(raw))
	}
	for _, line := range strings.Split(existingTodayBlock, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "##") {
			continue
		}
		addBullet(line)
	}
	pastHeader := false
	headerNorm := strings.TrimSpace(digestHeader)
	for _, line := range strings.Split(newSection, "\n") {
		t := strings.TrimSpace(line)
		if t == headerNorm {
			pastHeader = true
			continue
		}
		if strings.HasPrefix(t, "## ") {
			break
		}
		if !pastHeader {
			continue
		}
		addBullet(line)
	}
	var b strings.Builder
	b.WriteString(digestHeader)
	b.WriteByte('\n')
	if len(lines) == 0 {
		b.WriteString("- (no durable entries)\n")
	} else {
		for _, l := range lines {
			b.WriteString(l)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}
