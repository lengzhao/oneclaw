package builtin

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/tools/workspace"
)

const (
	NameWriteMemoryMonth  = "write_memory_month"
	NameAppendMemoryMonth = "append_memory_month"
	NameReadMemoryMonth   = "read_memory_month"
)

type memoryMonthPathIn struct {
	Path string `json:"path" jsonschema:"description=Exact relative path memory/2026-05/note.md (UTC yyyy-mm, single .md filename); use lowercase memory/ prefix"`
}

type writeMemoryMonthIn struct {
	Path    string `json:"path" jsonschema:"description=Exact relative path memory/2026-05/note.md under instruction root (UTC month folder)"`
	Content string `json:"content" jsonschema:"description=Full UTF-8 file contents"`
}

type appendMemoryMonthIn struct {
	Path    string `json:"path" jsonschema:"description=Exact relative path memory/2026-05/note.md under instruction root"`
	Content string `json:"content" jsonschema:"description=UTF-8 text to append"`
}

// InferWriteMemoryMonth writes only under InstructionRoot/memory/yyyy-mm/*.md.
func InferWriteMemoryMonth(instructionRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(instructionRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: instruction root required", NameWriteMemoryMonth)
	}
	mm := memory.MonthUTC(time.Now())
	return utils.InferTool(NameWriteMemoryMonth,
		fmt.Sprintf("Create or overwrite a markdown note under memory/%s/ relative to the session instruction root. The folder must be the current UTC month (%s).", mm, mm),
		func(ctx context.Context, in writeMemoryMonthIn) (string, error) {
			if len(in.Content) > workspace.MaxWorkspaceWriteBytes {
				return "", fmt.Errorf("content exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
			}
			if err := memory.RequireWriteUsesCurrentUTCMemoryMonth(in.Path, time.Now()); err != nil {
				return "", err
			}
			full, err := memory.ResolveMemoryMonthMarkdown(root, in.Path)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(full, []byte(in.Content), 0o644); err != nil {
				return "", err
			}
			return fmt.Sprintf("wrote %d bytes to %s", len(in.Content), filepath.ToSlash(strings.TrimSpace(in.Path))), nil
		})
}

// InferAppendMemoryMonth appends to memory/YYYY-MM/*.md under InstructionRoot.
func InferAppendMemoryMonth(instructionRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(instructionRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: instruction root required", NameAppendMemoryMonth)
	}
	mm := memory.MonthUTC(time.Now())
	return utils.InferTool(NameAppendMemoryMonth,
		fmt.Sprintf("Append UTF-8 text under memory/%s/. Creates the file if missing. The folder must be the current UTC month (%s).", mm, mm),
		func(ctx context.Context, in appendMemoryMonthIn) (string, error) {
			if len(in.Content) > workspace.MaxWorkspaceWriteBytes {
				return "", fmt.Errorf("content exceeds %d bytes", workspace.MaxWorkspaceWriteBytes)
			}
			if err := memory.RequireWriteUsesCurrentUTCMemoryMonth(in.Path, time.Now()); err != nil {
				return "", err
			}
			full, err := memory.ResolveMemoryMonthMarkdown(root, in.Path)
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
			return fmt.Sprintf("appended %d bytes to %s", n, filepath.ToSlash(strings.TrimSpace(in.Path))), nil
		})
}

// InferReadMemoryMonth reads only memory/YYYY-MM/*.md under InstructionRoot.
func InferReadMemoryMonth(instructionRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(instructionRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: instruction root required", NameReadMemoryMonth)
	}
	mm := memory.MonthUTC(time.Now())
	return utils.InferTool(NameReadMemoryMonth,
		fmt.Sprintf("Read a UTF-8 markdown file under memory/YYYY-MM/ relative to the session instruction root. Any valid UTC month is allowed (example current month: %s).", mm),
		func(ctx context.Context, in memoryMonthPathIn) (string, error) {
			full, err := memory.ResolveMemoryMonthMarkdown(root, in.Path)
			if err != nil {
				return "", err
			}
			b, err := os.ReadFile(full)
			if err != nil {
				if os.IsNotExist(err) {
					return "(no such file yet)", nil
				}
				return "", err
			}
			return string(b), nil
		})
}
