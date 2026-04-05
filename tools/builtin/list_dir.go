package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/pathutil"
	"github.com/openai/openai-go"
)

const listDirMaxEntries = 500

type listDirInput struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

// ListDirTool lists directory entries under the session working directory or memory roots.
type ListDirTool struct{}

func (ListDirTool) Name() string          { return "list_dir" }
func (ListDirTool) ConcurrencySafe() bool { return true }
func (ListDirTool) Description() string {
	return "List files and subdirectories under a path (cwd or memory roots, same as read_file). " +
		"Non-recursive lists one level; recursive walks the tree skipping .git, node_modules, vendor, and hidden directories. " +
		"Directories end with /. Max 500 entries."
}

func (ListDirTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Directory path relative to cwd or under memory roots",
		},
		"recursive": map[string]any{
			"type":        "boolean",
			"description": "If true, list all descendants (depth-first, sorted per directory)",
		},
	}, []string{"path"})
}

func (ListDirTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in listDirInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	rootAbs, err := pathutil.ResolveForSession(tctx.CWD, tctx.MemoryWriteRoots, in.Path)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(rootAbs)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("path is not a directory")
	}

	var lines []string
	if in.Recursive {
		lines, err = listDirRecursive(ctx, rootAbs)
	} else {
		lines, err = listDirOneLevel(rootAbs)
	}
	if err != nil {
		return "", err
	}
	sort.Strings(lines)
	truncated := false
	if len(lines) > listDirMaxEntries {
		truncated = true
		lines = lines[:listDirMaxEntries]
	}
	if len(lines) == 0 {
		return "(empty)", nil
	}
	out := strings.Join(lines, "\n")
	if truncated {
		out += fmt.Sprintf("\n\n[truncated: max %d entries]", listDirMaxEntries)
	}
	return out, nil
}

func listDirOneLevel(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var lines []string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			lines = append(lines, name+"/")
		} else {
			lines = append(lines, name)
		}
	}
	return lines, nil
}

func listDirRecursive(ctx context.Context, rootAbs string) ([]string, error) {
	var lines []string
	err := filepath.WalkDir(rootAbs, func(full string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if len(lines) >= listDirMaxEntries {
			return fs.SkipAll
		}
		if full == rootAbs {
			return nil
		}
		base := filepath.Base(full)
		if d.IsDir() {
			if shouldSkipWalkDir(full, rootAbs, base) {
				return fs.SkipDir
			}
		}
		rel, err := filepath.Rel(rootAbs, full)
		if err != nil {
			return nil
		}
		if strings.Contains(rel, string(filepath.Separator)+".") || strings.HasPrefix(rel, ".") {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if d.IsDir() {
			lines = append(lines, relSlash+"/")
		} else {
			lines = append(lines, relSlash)
		}
		return nil
	})
	return lines, err
}
