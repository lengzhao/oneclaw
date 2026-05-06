package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/skillsusage"
)

type readSkillIn struct {
	SkillID string `json:"skill_id" jsonschema:"description=Skill folder id under UserDataRoot/skills (for example: go-workflow)"`
	Path    string `json:"path,omitempty" jsonschema:"description=File path under skills/<skill_id>/, default SKILL.md"`
}

// InferReadSkill builds read_skill bound to UserDataRoot/skills.
func InferReadSkill(userDataRoot string) (tool.InvokableTool, error) {
	udr := filepath.Clean(strings.TrimSpace(userDataRoot))
	if udr == "" || udr == "." {
		return nil, fmt.Errorf("%s: user data root required", NameReadSkill)
	}
	desc := "Read one skill artifact from UserDataRoot/skills/<skill_id>/...; default file is SKILL.md."
	return utils.InferTool(NameReadSkill, desc, func(ctx context.Context, in readSkillIn) (string, error) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		sid := strings.TrimSpace(in.SkillID)
		if sid == "" {
			return "", fmt.Errorf("skill_id required")
		}
		relFile := strings.TrimSpace(in.Path)
		if relFile == "" {
			relFile = "SKILL.md"
		}
		relFile = filepath.ToSlash(strings.TrimPrefix(relFile, "./"))
		if strings.HasPrefix(relFile, "/") || strings.Contains(relFile, "..") {
			return "", fmt.Errorf("invalid path under skill folder")
		}
		rel := "skills/" + sid + "/" + relFile
		full, err := memory.ResolveSkillsMarkdown(udr, rel)
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		skillsRoot := filepath.Join(udr, "skills")
		_ = skillsusage.Record(skillsRoot, sid, NameReadSkill)
		return string(b), nil
	})
}
