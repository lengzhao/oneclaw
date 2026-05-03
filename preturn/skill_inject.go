package preturn

import (
	"path/filepath"
	"strings"
)

// NormalizeSkillRefs returns deduplicated, trimmed catalog skill ids.
// Empty output means the agent did not declare skills: — callers treat that as "all skills on disk" for digests.
func NormalizeSkillRefs(ids []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// ReferencedSkillsIndexMarkdown lists catalog referenced skills as "- id: description" (YAML description or body title).
// When catalog skills: is non-empty after normalization, SkillsDigestMarkdown lists only those ids
// (intersected with installed skills/ trees); otherwise it lists every skill under skills/.
func ReferencedSkillsIndexMarkdown(userDataRoot string, ids []string) string {
	ids = NormalizeSkillRefs(ids)
	if len(ids) == 0 {
		return ""
	}
	var lines []string
	for _, id := range ids {
		p := filepath.Join(userDataRoot, "skills", id, "SKILL.md")
		sum := skillOneLineSummary(p)
		lines = append(lines, "- "+id+": "+sum)
	}
	if len(lines) == 0 {
		return ""
	}
	return "## Referenced skills (index)\n\n" + strings.Join(lines, "\n")
}
