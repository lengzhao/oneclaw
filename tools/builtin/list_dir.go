package builtin

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/lengzhao/oneclaw/tools/workspace"
)

const maxListDirEntries = 2000

type listDirIn struct {
	Path      string `json:"path,omitempty" jsonschema:"description=Directory relative to workspace; omit or . for workspace root"`
	Recursive bool   `json:"recursive,omitempty" jsonschema:"description=If true, list files recursively (breadth via WalkDir)"`
}

// InferListDir builds the list_dir builtin bound to workspaceRoot.
func InferListDir(workspaceRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameListDir)
	}
	return utils.InferTool(NameListDir, "List files and directories under a path within the workspace. Lines ending with / are directories.",
		func(ctx context.Context, in listDirIn) (string, error) {
			rel := strings.TrimSpace(in.Path)
			if rel == "" {
				rel = "."
			}
			dir, err := workspace.ResolveUnderWorkspace(root, rel)
			if err != nil {
				return "", err
			}
			fi, err := os.Stat(dir)
			if err != nil {
				return "", err
			}
			if !fi.IsDir() {
				return "", fmt.Errorf("not a directory: %s", rel)
			}
			var lines []string
			n := 0
			if !in.Recursive {
				entries, err := os.ReadDir(dir)
				if err != nil {
					return "", err
				}
				for _, e := range entries {
					n++
					if n > maxListDirEntries {
						return "", fmt.Errorf("listing exceeds %d entries", maxListDirEntries)
					}
					name := e.Name()
					if e.IsDir() {
						lines = append(lines, name+"/")
					} else {
						lines = append(lines, name)
					}
				}
			} else {
				err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if path == dir {
						return nil
					}
					n++
					if n > maxListDirEntries {
						return fmt.Errorf("listing exceeds %d entries", maxListDirEntries)
					}
					relOut, err := filepath.Rel(dir, path)
					if err != nil {
						return err
					}
					relOut = filepath.ToSlash(relOut)
					if d.IsDir() {
						lines = append(lines, relOut+"/")
					} else {
						lines = append(lines, relOut)
					}
					return nil
				})
				if err != nil {
					return "", err
				}
			}
			slices.Sort(lines)
			if len(lines) == 0 {
				return "(empty directory)", nil
			}
			return strings.Join(lines, "\n"), nil
		})
}
