package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/lengzhao/oneclaw/memory"
)

type readFileIn struct {
	Path string `json:"path" jsonschema:"description=Path relative to workspace, instruction core file AGENT.md/MEMORY.md/SOUL.md/USER.md, instruction-root memory path, or user-data skills/<skill-id>/... path"`
}

var readFileMemoryMonthLeading = regexp.MustCompile(`(?i)^\d{4}-\d{2}/`)
var readFileMemoryDayFlat = regexp.MustCompile(`(?i)^\d{4}-\d{2}-\d{2}\.md$`)

type scopedPathKind int

const (
	scopedPathWorkspace scopedPathKind = iota
	scopedPathMemory
	scopedPathInstruction
	scopedPathSkills
)

func resolveAnyFileAbsScoped(workspaceRoot, instructionRoot, userDataRoot, userPath string) (abs string, kind scopedPathKind, err error) {
	raw := strings.TrimSpace(userPath)
	if raw == "" {
		return "", scopedPathWorkspace, fmt.Errorf("path required")
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw), scopedPathWorkspace, nil
	}
	rel := filepath.ToSlash(raw)
	rel = strings.TrimPrefix(rel, "./")
	lower := strings.ToLower(rel)
	useInstrMem := strings.HasPrefix(lower, "memory/") ||
		readFileMemoryDayFlat.MatchString(rel) ||
		readFileMemoryMonthLeading.MatchString(rel)
	if useInstrMem && strings.TrimSpace(instructionRoot) != "" {
		full, err := memory.ResolveMemoryMonthMarkdown(instructionRoot, rel)
		if err == nil {
			return full, scopedPathMemory, nil
		}
	}
	if full, ok, err := resolveInstructionCoreFileMaybe(instructionRoot, rel); ok {
		return full, scopedPathInstruction, err
	}
	if strings.HasPrefix(lower, "skills/") && strings.TrimSpace(userDataRoot) != "" {
		return filepath.Clean(filepath.Join(strings.TrimSpace(userDataRoot), filepath.FromSlash(rel))), scopedPathSkills, nil
	}
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return filepath.Clean(raw), scopedPathWorkspace, nil
	}
	return filepath.Clean(filepath.Join(root, filepath.FromSlash(rel))), scopedPathWorkspace, nil
}

func resolveReadFileAbsScoped(workspaceRoot, instructionRoot, userDataRoot, userPath string) (abs string, err error) {
	full, _, err := resolveAnyFileAbsScoped(workspaceRoot, instructionRoot, userDataRoot, userPath)
	return full, err
}

// InferReadFile builds the read_file builtin bound to workspaceRoot only (tests / narrow callers).
func InferReadFile(workspaceRoot string) (tool.InvokableTool, error) {
	return InferReadFileWorkspaceAndMemory(workspaceRoot, "")
}

// InferReadFileWorkspaceAndMemory binds read_file to the workspace plus instruction-root memory/*.md (when instructionRoot is non-empty).
func InferReadFileWorkspaceAndMemory(workspaceRoot, instructionRoot string) (tool.InvokableTool, error) {
	return InferReadFileScoped(workspaceRoot, instructionRoot, "")
}

// InferReadFileScoped binds read_file to workspace plus scoped instruction and user-data paths.
func InferReadFileScoped(workspaceRoot, instructionRoot, userDataRoot string) (tool.InvokableTool, error) {
	ws := strings.TrimSpace(workspaceRoot)
	if ws == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameReadFile)
	}
	desc := "Read a UTF-8 text file from any absolute path, or from a path relative to the workspace."
	if strings.TrimSpace(instructionRoot) != "" {
		desc += " AGENT.md, MEMORY.md, SOUL.md, USER.md, paths starting with memory/, or shaped like <yyyy-mm>/… / <yyyy-mm-dd>.md, resolve under the instruction root."
	}
	if strings.TrimSpace(userDataRoot) != "" {
		desc += " Paths starting with skills/ resolve under the user data root."
	}
	return utils.InferTool(NameReadFile, desc, func(ctx context.Context, in readFileIn) (string, error) {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		full, err := resolveReadFileAbsScoped(ws, instructionRoot, userDataRoot, in.Path)
		if err != nil {
			return "", err
		}
		b, err := os.ReadFile(full)
		if err != nil {
			return "", err
		}
		return string(b), nil
	})
}
