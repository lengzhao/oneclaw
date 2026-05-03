package builtin

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const maxGlobMatches = 2000

type globIn struct {
	Pattern string `json:"pattern" jsonschema:"description=Glob pattern relative to workspace. Use forward slashes. Syntax is filepath.Match (* and ?); ** is not special."`
	// Recursive matches pattern against every file path relative to workspace (slash-separated). When false, pattern is passed to filepath.Glob under workspace (typically one directory level unless pattern contains subdirs).
	Recursive bool `json:"recursive,omitempty" jsonschema:"description=If true, walk all files under workspace and match pattern against each relative path"`
}

// InferGlob builds the glob builtin bound to workspaceRoot.
func InferGlob(workspaceRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(workspaceRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: workspace root required", NameGlob)
	}
	absRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, err
	}
	return utils.InferTool(NameGlob, "List files under the workspace matching a glob pattern (read-only). Non-recursive: filepath.Glob under workspace. Recursive: walk all files; if pattern contains '/', match against relative path, else match against file basename (e.g. *.txt matches any depth). Sorted slash-separated paths.",
		func(ctx context.Context, in globIn) (string, error) {
			pat := strings.TrimSpace(in.Pattern)
			if pat == "" {
				return "", fmt.Errorf("pattern required")
			}
			if strings.Contains(pat, "..") {
				return "", fmt.Errorf("invalid pattern")
			}
			patSlash := filepath.ToSlash(pat)
			var matches []string
			if in.Recursive {
				walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, entryErr error) error {
					if entryErr != nil {
						return entryErr
					}
					if d.IsDir() {
						return nil
					}
					rel, relErr := filepath.Rel(absRoot, path)
					if relErr != nil {
						return relErr
					}
					relSlash := filepath.ToSlash(rel)
					var ok bool
					var matchErr error
					if strings.Contains(patSlash, "/") {
						ok, matchErr = filepath.Match(patSlash, relSlash)
					} else {
						ok, matchErr = filepath.Match(patSlash, filepath.Base(relSlash))
					}
					if matchErr != nil {
						return matchErr
					}
					if !ok {
						return nil
					}
					if len(matches) >= maxGlobMatches {
						return fmt.Errorf("glob exceeds %d matches", maxGlobMatches)
					}
					matches = append(matches, relSlash)
					return nil
				})
				if walkErr != nil {
					return "", walkErr
				}
			} else {
				joined := filepath.Join(absRoot, filepath.FromSlash(patSlash))
				joinedAbs, err := filepath.Abs(filepath.Clean(joined))
				if err != nil {
					return "", err
				}
				relToRoot, err := filepath.Rel(absRoot, joinedAbs)
				if err != nil {
					return "", fmt.Errorf("pattern escapes workspace")
				}
				if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
					return "", fmt.Errorf("pattern escapes workspace")
				}
				raw, err := filepath.Glob(joinedAbs)
				if err != nil {
					return "", err
				}
				for _, m := range raw {
					rel, err := filepath.Rel(absRoot, m)
					if err != nil {
						return "", err
					}
					if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
						continue
					}
					if len(matches) >= maxGlobMatches {
						return "", fmt.Errorf("glob exceeds %d matches", maxGlobMatches)
					}
					matches = append(matches, filepath.ToSlash(rel))
				}
			}
			slices.Sort(matches)
			if len(matches) == 0 {
				return "(no matches)", nil
			}
			return strings.Join(matches, "\n"), nil
		})
}
