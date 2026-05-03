package preturn

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/skillsusage"
)

const skillsDigestHotN = 20

// SkillsDigestMarkdown lists skills under skillsRoot with hot ranking (exported for workflow list_skills).
// catalogSkillIDs is the agent's YAML skills: list (see NormalizeSkillRefs). When non-empty, only those
// ids that exist on disk are listed; when empty, every installed skill under skills/ is listed.
func SkillsDigestMarkdown(skillsRoot string, budget Budget, catalogSkillIDs []string) (string, error) {
	return skillsDigest(skillsRoot, budget, catalogSkillIDs)
}

func skillsDigest(skillsRoot string, budget Budget, catalogSkillIDs []string) (string, error) {
	if _, err := os.Stat(skillsRoot); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	restrict := NormalizeSkillRefs(catalogSkillIDs)
	discovered, err := discoverSkillIDs(skillsRoot)
	if err != nil {
		return "", err
	}
	if len(discovered) == 0 && len(restrict) == 0 {
		return "", nil
	}
	installed := make(map[string]bool, len(discovered))
	for _, id := range discovered {
		installed[id] = true
	}
	var allIDs []string
	if len(restrict) > 0 {
		for _, id := range restrict {
			if installed[id] {
				allIDs = append(allIDs, id)
			}
		}
		if len(allIDs) == 0 {
			return "(Catalog references skills that are not installed yet — add skills/<id>/SKILL.md under user-data.)", nil
		}
	} else {
		allIDs = discovered
		if len(allIDs) == 0 {
			return "", nil
		}
	}
	counts, lastUsed, err := skillsusage.Aggregate(skillsRoot)
	if err != nil {
		return "", err
	}
	ranked := skillsusage.RankSkillIDs(counts, lastUsed, allIDs)
	hot := skillsDigestHotN
	if hot > len(ranked) {
		hot = len(ranked)
	}
	var lines []string
	header := "Hot skills (top " + strconv.Itoa(skillsDigestHotN) + " by usage; log skills/" + skillsusage.LogFileName + "):"
	if len(restrict) > 0 {
		header = "Skills for this agent (catalog allowlist; log skills/" + skillsusage.LogFileName + "):"
	}
	lines = append(lines, header)
	for _, id := range ranked[:hot] {
		skillMd := filepath.Join(skillsRoot, id, "SKILL.md")
		sum := skillOneLineSummary(skillMd)
		lines = append(lines, "- "+id+": "+sum)
	}
	var rest []string
	for _, id := range ranked[hot:] {
		rest = append(rest, id)
	}
	sort.Strings(rest)
	if len(rest) > 0 {
		lines = append(lines, "")
		lines = append(lines, "Other skills:")
		for _, id := range rest {
			skillMd := filepath.Join(skillsRoot, id, "SKILL.md")
			sum := skillOneLineSummary(skillMd)
			lines = append(lines, "- "+id+": "+sum)
		}
	}
	out := strings.Join(lines, "\n")
	max := budget.SkillsMaxRunes
	if max <= 0 {
		max = DefaultBudget().SkillsMaxRunes
	}
	return truncateRunes(out, max), nil
}

func discoverSkillIDs(skillsRoot string) ([]string, error) {
	entries, err := os.ReadDir(skillsRoot)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, "_") || strings.HasPrefix(name, ".") {
			continue
		}
		skillMd := filepath.Join(skillsRoot, name, "SKILL.md")
		st, err := os.Stat(skillMd)
		if err != nil || st.IsDir() {
			continue
		}
		ids = append(ids, name)
	}
	sort.Strings(ids)
	return ids, nil
}

func skillOneLineSummary(skillMdPath string) string {
	b, err := os.ReadFile(skillMdPath)
	if err != nil || len(b) == 0 {
		return "(no description)"
	}
	return skillOneLineSummaryFromBytes(b)
}

func skillOneLineSummaryFromBytes(b []byte) string {
	fmBytes, body, err := catalog.SplitYAMLFrontmatter(b)
	if err == nil && len(bytes.TrimSpace(fmBytes)) > 0 {
		var fm struct {
			Description string `yaml:"description,omitempty"`
		}
		if yaml.Unmarshal(fmBytes, &fm) == nil {
			if d := strings.TrimSpace(fm.Description); d != "" {
				return collapseSkillSummaryLine(d)
			}
		}
		return collapseSkillSummaryLine(skillTitleFromBody(body))
	}
	return collapseSkillSummaryLine(skillTitleFromBody(strings.TrimSpace(string(b))))
}

func collapseSkillSummaryLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.Join(strings.Fields(s), " ")
}

func skillTitleFromBody(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(no description)"
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "#"), " "))
		}
		return line
	}
	return "(no description)"
}
