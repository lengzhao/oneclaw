package skills

import (
	"os"
	"strings"
)

// Skill is one loaded capability under <root>/<name>/SKILL.md.
type Skill struct {
	Name        string
	RootDir     string
	FilePath    string
	Description string
	WhenToUse   string
}

// PromptBody reads the current file, strips frontmatter, and prefixes the skill root (Claude Code–style).
func (s Skill) PromptBody() (string, error) {
	raw, err := os.ReadFile(s.FilePath)
	if err != nil {
		return "", err
	}
	_, body := ParseFrontmatter(string(raw))
	body = strings.TrimSpace(body)
	var b strings.Builder
	b.WriteString("Base directory for this skill: ")
	b.WriteString(s.RootDir)
	b.WriteString("\n\n")
	b.WriteString(body)
	return b.String(), nil
}
