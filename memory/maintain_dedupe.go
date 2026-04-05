package memory

import (
	"regexp"
	"strings"
	"unicode"
)

var bulletLine = regexp.MustCompile(`^\s*[-*]\s+(.*)$`)

// normalizeBulletKey collapses whitespace and lowercases for near-duplicate detection.
func normalizeBulletKey(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	lastSpace := false
	for _, r := range strings.ToLower(s) {
		if unicode.IsSpace(r) {
			if !lastSpace && b.Len() > 0 {
				b.WriteRune(' ')
				lastSpace = true
			}
			continue
		}
		lastSpace = false
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

// bulletKeysFromMarkdown scans markdown for bullet lines and returns normalized keys.
func bulletKeysFromMarkdown(md string) map[string]struct{} {
	out := make(map[string]struct{})
	for _, line := range strings.Split(md, "\n") {
		m := bulletLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		k := normalizeBulletKey(m[1])
		if k != "" {
			out[k] = struct{}{}
		}
	}
	return out
}

// dedupeMaintenanceBullets removes bullets whose normalized text already appears in
// existingCorpus or earlier in the same section (strong dedupe for model output).
func dedupeMaintenanceBullets(section, existingCorpus string) string {
	section = strings.TrimSpace(section)
	if section == "" {
		return section
	}
	lines := strings.Split(section, "\n")
	if len(lines) == 0 {
		return section
	}
	seen := bulletKeysFromMarkdown(existingCorpus)
	var head []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		if strings.TrimSpace(line) != "" {
			head = append(head, line)
			i++
			break
		}
		i++
	}
	if i >= len(lines) {
		return strings.Join(head, "\n")
	}
	var bullets []string
	localSeen := make(map[string]struct{})
	for ; i < len(lines); i++ {
		raw := lines[i]
		trim := strings.TrimSpace(raw)
		if trim == "" {
			continue
		}
		m := bulletLine.FindStringSubmatch(raw)
		if m == nil {
			continue
		}
		k := normalizeBulletKey(m[1])
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		if _, ok := localSeen[k]; ok {
			continue
		}
		localSeen[k] = struct{}{}
		seen[k] = struct{}{}
		bullets = append(bullets, raw)
	}
	var b strings.Builder
	for _, h := range head {
		b.WriteString(h)
		b.WriteByte('\n')
	}
	if len(bullets) == 0 {
		b.WriteString("- (no durable entries)\n")
	} else {
		for _, bl := range bullets {
			b.WriteString(bl)
			b.WriteByte('\n')
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// maintenanceSectionOnlyNoDurable is true when the section has no substantive bullets after dedupe.
func maintenanceSectionOnlyNoDurable(section string) bool {
	section = strings.TrimSpace(section)
	for _, line := range strings.Split(section, "\n") {
		trim := strings.TrimSpace(line)
		if trim == "" {
			continue
		}
		if strings.HasPrefix(trim, "##") {
			continue
		}
		m := bulletLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		inner := strings.ToLower(strings.TrimSpace(m[1]))
		if inner == "(no durable entries)" {
			return true
		}
		return false
	}
	return false
}
