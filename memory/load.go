package memory

import (
	"os"
	"strings"
)

// StripYAMLFrontmatter removes a leading --- ... --- block if present.
func StripYAMLFrontmatter(raw string) string {
	s := strings.TrimPrefix(raw, "\ufeff")
	if !strings.HasPrefix(s, "---\n") {
		return s
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return s
	}
	after := rest[end+4:]
	if strings.HasPrefix(after, "\n") {
		return after[1:]
	}
	if strings.HasPrefix(after, "\r\n") {
		return after[2:]
	}
	return after
}

// LoadMarkdownBody reads a text/markdown file, strips optional YAML frontmatter, and trims whitespace.
func LoadMarkdownBody(abs string) string {
	if !IsTextExtension(abs) {
		return ""
	}
	b, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(StripYAMLFrontmatter(string(b)))
}
