package memory

import (
	"path/filepath"
	"strings"
)

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
