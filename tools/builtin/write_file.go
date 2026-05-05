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

type writeFileIn struct {
	Path      string `json:"path" jsonschema:"description=Absolute file path, path relative to workspace, AGENT.md/MEMORY.md/SOUL.md/USER.md, memory/<yyyy-mm>/<file>.md, or skills/<skill-id>/..."`
	Content   string `json:"content" jsonschema:"description=UTF-8 text to write, append, or use as replacement text"`
	Operation string `json:"operation,omitempty" jsonschema:"description=write (default), append, or replace"`
	OldText   string `json:"old_text,omitempty" jsonschema:"description=Exact text to replace when operation=replace; must occur exactly once"`
}

// InferWriteFile builds the write_file builtin bound to workspaceRoot.
func InferWriteFile(workspaceRoot string) (tool.InvokableTool, error) {
	return InferWriteFileScoped(workspaceRoot, "", "")
}

// InferWriteFileScoped builds a single write_file tool for workspace, instruction memory/core files, and skills artifacts.
func InferWriteFileScoped(workspaceRoot, instructionRoot, userDataRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameWriteFile)
	}
	desc := "Create, append, or exact-replace UTF-8 text files at any absolute path, or a path relative to the workspace."
	if strings.TrimSpace(instructionRoot) != "" {
		desc += " AGENT.md, MEMORY.md, SOUL.md, USER.md, and memory/<yyyy-mm>/*.md resolve under the instruction root."
	}
	if strings.TrimSpace(userDataRoot) != "" {
		desc += " skills/<skill-id>/... resolves under the user data root and records skill usage."
	}
	return utils.InferTool(NameWriteFile, desc,
		func(ctx context.Context, in writeFileIn) (string, error) {
			if strings.TrimSpace(in.Path) == "" {
				return "", fmt.Errorf("path required")
			}
			if len(in.Content) > workspace.MaxWorkspaceWriteBytes {
				return "", fmt.Errorf("content exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
			}
			full, kind, err := resolveAnyFileAbsScoped(root, instructionRoot, userDataRoot, in.Path)
			if err != nil {
				return "", err
			}
			op := strings.ToLower(strings.TrimSpace(in.Operation))
			if op == "" {
				op = "write"
			}
			switch op {
			case "write":
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					return "", err
				}
				if err := os.WriteFile(full, []byte(in.Content), 0o644); err != nil {
					return "", err
				}
			case "append":
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					return "", err
				}
				f, err := os.OpenFile(full, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
				if err != nil {
					return "", err
				}
				defer f.Close()
				if _, err := f.WriteString(in.Content); err != nil {
					return "", err
				}
			case "replace":
				if in.OldText == "" {
					return "", fmt.Errorf("old_text required for replace")
				}
				raw, err := os.ReadFile(full)
				if err != nil {
					return "", err
				}
				if len(raw) > workspace.MaxWorkspaceWriteBytes {
					return "", fmt.Errorf("file exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
				}
				body := string(raw)
				n := strings.Count(body, in.OldText)
				if n == 0 {
					return "", fmt.Errorf("old_text not found")
				}
				if n > 1 {
					return "", fmt.Errorf("old_text matches %d times; must match exactly once", n)
				}
				out := strings.Replace(body, in.OldText, in.Content, 1)
				if len(out) > workspace.MaxWorkspaceWriteBytes {
					return "", fmt.Errorf("result exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
				}
				if err := os.WriteFile(full, []byte(out), 0o644); err != nil {
					return "", err
				}
			default:
				return "", fmt.Errorf("operation must be write, append, or replace")
			}
			if kind == scopedPathSkills {
				skillRoot := filepath.Join(strings.TrimSpace(userDataRoot), "skills")
				if sid, err := memory.SkillIDFromSkillsRel(in.Path); err == nil {
					_ = skillsusage.Record(skillRoot, sid, NameWriteFile)
				}
			}
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			if _, err := os.Stat(full); err != nil {
				return "", err
			}
			return fmt.Sprintf("%s %d bytes at %s", op, len(in.Content), filepath.ToSlash(strings.TrimSpace(in.Path))), nil
		})
}
