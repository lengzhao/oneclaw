package catalog

import (
	"embed"
	"io/fs"
	pathpkg "path"
	"strings"
)

//go:embed all:builtin/skills
var builtinSkillsFS embed.FS

const builtinSkillsPrefix = "builtin/skills"

// ReadBuiltinSkillSKILL returns embedded SKILL.md for skillID (e.g. "skill-creator").
func ReadBuiltinSkillSKILL(skillID string) ([]byte, error) {
	skillID = strings.TrimSpace(skillID)
	if skillID == "" || strings.Contains(skillID, "..") || strings.Contains(skillID, "/") || strings.Contains(skillID, "\\") {
		return nil, fs.ErrNotExist
	}
	p := pathpkg.Join(builtinSkillsPrefix, skillID, "SKILL.md")
	return builtinSkillsFS.ReadFile(p)
}
