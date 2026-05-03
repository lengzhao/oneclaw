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
	"github.com/lengzhao/oneclaw/tools/workspace"
)

const (
	NameWriteSkillFile  = "write_skill_file"
	NameAppendSkillFile = "append_skill_file"
)

type writeSkillFileIn struct {
	Path    string `json:"path" jsonschema:"description=Relative path skills/<skill-id>/file.ext"`
	Content string `json:"content" jsonschema:"description=Full UTF-8 file contents"`
}

type appendSkillFileIn struct {
	Path    string `json:"path" jsonschema:"description=Relative path skills/<skill-id>/file.ext"`
	Content string `json:"content" jsonschema:"description=UTF-8 text to append"`
}

// InferWriteSkillFile writes only under UserDataRoot/skills/<skill-id>/... (allowed extensions).
func InferWriteSkillFile(userDataRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(userDataRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: user data root required", NameWriteSkillFile)
	}
	return utils.InferTool(NameWriteSkillFile,
		"Create or overwrite a file under skills/<skill-id>/ (SKILL.md, scripts/*, reference/*). Allowed extensions only; paths relative to user data root.",
		func(ctx context.Context, in writeSkillFileIn) (string, error) {
			if len(in.Content) > workspace.MaxWorkspaceWriteBytes {
				return "", fmt.Errorf("content exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
			}
			full, err := memory.ResolveSkillsMarkdown(root, in.Path)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(full, []byte(in.Content), 0o644); err != nil {
				return "", err
			}
			skillRoot := filepath.Join(root, "skills")
			if sid, err := memory.SkillIDFromSkillsRel(in.Path); err == nil {
				_ = skillsusage.Record(skillRoot, sid, NameWriteSkillFile)
			}
			return fmt.Sprintf("wrote %d bytes to %s", len(in.Content), filepath.ToSlash(strings.TrimSpace(in.Path))), nil
		})
}

// InferAppendSkillFile appends under UserDataRoot/skills/<skill-id>/... (allowed extensions).
func InferAppendSkillFile(userDataRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(userDataRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: user data root required", NameAppendSkillFile)
	}
	return utils.InferTool(NameAppendSkillFile,
		"Append UTF-8 text under skills/<skill-id>/ (same path rules as write_skill_file). Creates the file if missing.",
		func(ctx context.Context, in appendSkillFileIn) (string, error) {
			if len(in.Content) > workspace.MaxWorkspaceWriteBytes {
				return "", fmt.Errorf("content exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
			}
			full, err := memory.ResolveSkillsMarkdown(root, in.Path)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", err
			}
			f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
			if err != nil {
				return "", err
			}
			defer f.Close()
			n, err := f.WriteString(in.Content)
			if err != nil {
				return "", err
			}
			skillRoot := filepath.Join(root, "skills")
			if sid, err := memory.SkillIDFromSkillsRel(in.Path); err == nil {
				_ = skillsusage.Record(skillRoot, sid, NameAppendSkillFile)
			}
			return fmt.Sprintf("appended %d bytes to %s", n, filepath.ToSlash(strings.TrimSpace(in.Path))), nil
		})
}
