package builtin

import (
	"context"
	"fmt"
	"log/slog"
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
	Path string `json:"path" jsonschema:"description=Optional relative path under memory/YYYY-MM/*.md. Empty defaults to memory/<UTC-yyyy-mm>/<UTC-yyyy-mm-dd>.md"`
}

type writeMemoryMonthIn struct {
	Path    string `json:"path" jsonschema:"description=Optional relative path under memory/YYYY-MM/*.md. Empty defaults to memory/<UTC-yyyy-mm>/<UTC-yyyy-mm-dd>.md"`
	Content string `json:"content" jsonschema:"description=Full UTF-8 file contents"`
}

type appendMemoryMonthIn struct {
	Path    string `json:"path" jsonschema:"description=Optional relative path under memory/YYYY-MM/*.md. Empty defaults to memory/<UTC-yyyy-mm>/<UTC-yyyy-mm-dd>.md"`
	Content string `json:"content" jsonschema:"description=UTF-8 text to append"`
}

func defaultMemoryMonthPath(now time.Time) string {
	u := now.UTC()
	return "memory/" + memory.MonthUTC(u) + "/" + u.Format("2006-01-02") + ".md"
}

func normalizeWritePathOrDefault(ctx context.Context, raw string, now time.Time) string {
	def := defaultMemoryMonthPath(now)
	rel := strings.TrimSpace(raw)
	if rel == "" {
		return def
	}
	norm, err := memory.NormalizeMemoryMonthRel(rel)
	if err == nil {
		if err2 := memory.RequireWriteUsesCurrentUTCMemoryMonth(norm, now); err2 == nil {
			return norm
		}
	}
	slog.WarnContext(ctx, "memory_month: invalid write path, fallback to default",
		"raw_path", rel,
		"default_path", def,
	)
	return def
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
			now := time.Now().UTC()
			rel := normalizeWritePathOrDefault(ctx, in.Path, now)
			if err := memory.RequireWriteUsesCurrentUTCMemoryMonth(rel, now); err != nil {
				return "", err
			}
			full, err := memory.ResolveMemoryMonthMarkdown(root, rel)
			if err != nil {
				return "", err
			}
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				return "", err
			}
			if err := os.WriteFile(full, []byte(in.Content), 0o644); err != nil {
				return "", err
			}
			return fmt.Sprintf("wrote %d bytes to %s", len(in.Content), filepath.ToSlash(rel)), nil
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
			now := time.Now().UTC()
			rel := normalizeWritePathOrDefault(ctx, in.Path, now)
			if err := memory.RequireWriteUsesCurrentUTCMemoryMonth(rel, now); err != nil {
				return "", err
			}
			full, err := memory.ResolveMemoryMonthMarkdown(root, rel)
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
			return fmt.Sprintf("appended %d bytes to %s", n, filepath.ToSlash(rel)), nil
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
			rel := strings.TrimSpace(in.Path)
			if rel == "" {
				rel = defaultMemoryMonthPath(time.Now().UTC())
			}
			full, err := memory.ResolveMemoryMonthMarkdown(root, rel)
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
