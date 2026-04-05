package skills

import (
	"fmt"
	"os"
	"sort"
	"strings"
)

// OrderSkills returns skills: recent-used first (still present in set), then the rest by name ascending.
func OrderSkills(all []Skill, recentNames []string) []Skill {
	byName := make(map[string]Skill, len(all))
	for _, s := range all {
		byName[s.Name] = s
	}
	var out []Skill
	seen := make(map[string]struct{})
	for _, n := range recentNames {
		if s, ok := byName[n]; ok {
			out = append(out, s)
			seen[n] = struct{}{}
		}
	}
	var tail []string
	for n := range byName {
		if _, ok := seen[n]; !ok {
			tail = append(tail, n)
		}
	}
	sort.Strings(tail)
	for _, n := range tail {
		out = append(out, byName[n])
	}
	return out
}

func listingDescription(s Skill) string {
	desc := strings.TrimSpace(s.Description)
	when := strings.TrimSpace(s.WhenToUse)
	switch {
	case desc != "" && when != "":
		desc = desc + " — " + when
	case desc == "" && when != "":
		desc = when
	case desc == "":
		desc = "(no description in frontmatter)"
	}
	runes := []rune(desc)
	if len(runes) > MaxListingDescChars {
		desc = string(runes[:MaxListingDescChars-1]) + "…"
	}
	return desc
}

func lineFull(s Skill) string {
	return fmt.Sprintf("- %s: %s", s.Name, listingDescription(s))
}

func lineNameOnly(s Skill) string {
	return "- " + s.Name
}

// FormatIndexLines returns one bullet line per skill under maxBytes (same picking rules as FormatIndex).
func FormatIndexLines(ordered []Skill, maxBytes int) []string {
	if maxBytes <= 0 || len(ordered) == 0 {
		return nil
	}
	var lines []string
	used := 0
	for _, s := range ordered {
		full := lineFull(s)
		short := lineNameOnly(s)
		var pick string
		if used+len(full)+1 <= maxBytes {
			pick = full
		} else if used+len(short)+1 <= maxBytes {
			pick = short
		} else {
			break
		}
		if len(lines) > 0 {
			used++
		}
		used += len(pick)
		lines = append(lines, pick)
	}
	return lines
}

// FormatIndex builds the skill listing under a byte budget (UTF-8 byte length, not runes).
func FormatIndex(ordered []Skill, maxBytes int) string {
	lines := FormatIndexLines(ordered, maxBytes)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

// PromptSkillLines returns skill bullet lines for system prompts (empty if disabled or no skills).
func PromptSkillLines(cwd, home string, maxBytes int) []string {
	if strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_SKILLS")) == "1" {
		return nil
	}
	all := LoadAll(cwd, home)
	if len(all) == 0 {
		return nil
	}
	rec, err := LoadRecent(RecentFilePath(cwd))
	if err != nil {
		rec = RecentFile{Version: 1}
	}
	ordered := OrderSkills(all, rec.NamesInOrder())
	return FormatIndexLines(ordered, maxBytes)
}

// SystemBlock returns a markdown section for the system prompt, or empty if disabled / no skills.
func SystemBlock(cwd, home string, maxBytes int) string {
	lines := PromptSkillLines(cwd, home, maxBytes)
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n## Skills\n\n")
	sb.WriteString("When a task matches a skill, call **invoke_skill** with that skill's name to load its full instructions (body of SKILL.md).\n\n")
	sb.WriteString(strings.Join(lines, "\n"))
	sb.WriteByte('\n')
	return sb.String()
}
