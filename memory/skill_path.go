package memory

import (
	"fmt"
	"strings"
)

// SkillIDFromSkillsRel returns the first path segment under skills/ (the skill folder id).
// Example: skills/foo/bar.md -> foo
func SkillIDFromSkillsRel(rel string) (string, error) {
	rel = strings.TrimSpace(filepathToSlash(rel))
	if rel == "" || strings.Contains(rel, "..") {
		return "", fmt.Errorf("memory: invalid skills path")
	}
	if !strings.HasPrefix(rel, "skills/") {
		return "", fmt.Errorf("memory: not under skills/")
	}
	rest := strings.TrimPrefix(rel, "skills/")
	if rest == "" {
		return "", fmt.Errorf("memory: empty skills path")
	}
	i := strings.Index(rest, "/")
	if i <= 0 {
		return "", fmt.Errorf("memory: path must be skills/<skill-id>/...")
	}
	id := rest[:i]
	if !validSkillFolderName(id) {
		return "", fmt.Errorf("memory: invalid skill id %q", id)
	}
	return id, nil
}

func filepathToSlash(s string) string {
	return strings.TrimSpace(strings.ReplaceAll(s, "\\", "/"))
}
