package skills

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter holds YAML between --- lines at the start of SKILL.md.
type Frontmatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	WhenToUse   string `yaml:"when_to_use"`
}

// ParseFrontmatter splits leading YAML frontmatter from markdown body.
func ParseFrontmatter(content string) (Frontmatter, string) {
	var empty Frontmatter
	s := strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(s, "---") {
		return empty, s
	}
	rest := strings.TrimPrefix(s, "---")
	rest = strings.TrimLeft(rest, "\n")
	idx := strings.Index(rest, "\n---")
	if idx < 0 {
		return empty, s
	}
	yamlBlock := rest[:idx]
	body := strings.TrimSpace(rest[idx+len("\n---"):])
	body = strings.TrimLeft(body, "\n")

	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(yamlBlock), &fm); err != nil {
		// Still expose body so the skill file can be invoked; listing metadata may be empty.
		return empty, body
	}
	return fm, body
}
