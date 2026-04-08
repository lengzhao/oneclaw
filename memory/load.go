package memory

import (
	"os"
	"path/filepath"
	"strings"
)

// StripYAMLFrontmatter removes a leading --- ... --- block if present.
func StripYAMLFrontmatter(raw string) string {
	start := BodyStartByteOffset(raw)
	if start > len(raw) {
		return ""
	}
	return raw[start:]
}

// BodyStartByteOffset returns the byte offset in raw where the markdown body begins
// (after optional UTF-8 BOM and optional YAML frontmatter). Matches StripYAMLFrontmatter slicing.
func BodyStartByteOffset(raw string) int {
	s := strings.TrimPrefix(raw, "\ufeff")
	bomSkip := len(raw) - len(s)
	if !strings.HasPrefix(s, "---\n") {
		return bomSkip
	}
	rest := s[4:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return bomSkip
	}
	after := rest[end+4:]
	base := bomSkip + 4 + end + 4
	if strings.HasPrefix(after, "\n") {
		return base + 1
	}
	if strings.HasPrefix(after, "\r\n") {
		return base + 2
	}
	return base
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

// IsTextExtension returns true for extensions we treat as text when loading instruction/markdown files.
func IsTextExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".md", ".txt", ".text", ".json", ".yaml", ".yml", ".toml", ".xml", ".csv",
		".html", ".htm", ".css", ".scss", ".go", ".ts", ".tsx", ".js", ".jsx", ".py",
		".sh", ".bash", ".zsh", ".sql", ".env", ".ini", ".log", ".diff", ".patch":
		return true
	default:
		return ext == ""
	}
}
