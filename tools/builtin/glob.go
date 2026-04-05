package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/pathutil"
	"github.com/openai/openai-go"
)

const globMaxResults = 500

type globInput struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
}

// GlobTool lists files under a directory matching a glob pattern relative to that directory.
// Pattern uses slash separators; supports * and ? per path segment and ** for recursive depth.
type GlobTool struct{}

func (GlobTool) Name() string          { return "glob" }
func (GlobTool) ConcurrencySafe() bool { return true }
func (GlobTool) Description() string {
	return "List file paths under a directory (cwd or memory roots; same path rules as read_file). " +
		"Pattern is relative to that directory, e.g. \"*.go\", \"src/*.go\", or \"**/*.go\". " +
		"Skips .git, node_modules, vendor, and hidden directories when walking. Max 500 paths."
}

func (GlobTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"path": map[string]any{
			"type":        "string",
			"description": "Directory to search (relative to cwd or under memory roots)",
		},
		"pattern": map[string]any{
			"type":        "string",
			"description": "Glob relative to path: *, ?, ** supported; must not contain ..",
		},
	}, []string{"path", "pattern"})
}

func (GlobTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in globInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	pat := strings.TrimSpace(in.Pattern)
	if pat == "" {
		return "", fmt.Errorf("pattern is required")
	}
	if strings.Contains(pat, "..") {
		return "", fmt.Errorf("pattern must not contain ..")
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

	var matches []string
	if strings.Contains(pat, "**") {
		matches, err = globRecursive(ctx, rootAbs, pat)
	} else {
		matches, err = globShallow(rootAbs, pat)
	}
	if err != nil {
		return "", err
	}
	sort.Strings(matches)
	if len(matches) >= globMaxResults {
		matches = matches[:globMaxResults]
		matches = append(matches, fmt.Sprintf("\n[truncated: max %d paths]", globMaxResults))
	}
	if len(matches) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(matches, "\n"), nil
}

func globShallow(rootAbs, pattern string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(rootAbs, filepath.FromSlash(pattern)))
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(matches))
	for _, p := range matches {
		if ok, err := fileUnderRoot(rootAbs, p); err != nil || !ok {
			continue
		}
		fi, err := os.Stat(p)
		if err != nil {
			continue
		}
		if fi.IsDir() {
			continue
		}
		rel, err := filepath.Rel(rootAbs, p)
		if err != nil {
			continue
		}
		out = append(out, filepath.ToSlash(rel))
	}
	return out, nil
}

func fileUnderRoot(root, p string) (bool, error) {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	pAbs, err := filepath.Abs(p)
	if err != nil {
		return false, err
	}
	rel, err := filepath.Rel(rootAbs, pAbs)
	if err != nil {
		return false, nil
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false, nil
	}
	return true, nil
}

func globRecursive(ctx context.Context, rootAbs, pattern string) ([]string, error) {
	patSlash := filepath.ToSlash(filepath.Clean(pattern))
	var out []string
	err := filepath.WalkDir(rootAbs, func(full string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if len(out) >= globMaxResults {
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
			return nil
		}
		rel, err := filepath.Rel(rootAbs, full)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if matchPathPattern(relSlash, patSlash) {
			out = append(out, relSlash)
		}
		return nil
	})
	return out, err
}

func shouldSkipWalkDir(full, rootAbs, base string) bool {
	if full == rootAbs {
		return false
	}
	if strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor" {
		return true
	}
	return false
}

// matchPathPattern matches relative path rel (slash-separated) against pattern (slash-separated).
func matchPathPattern(rel, pat string) bool {
	r := strings.Split(rel, "/")
	p := strings.Split(pat, "/")
	var match func(ri, pi int) bool
	match = func(ri, pi int) bool {
		if pi == len(p) {
			return ri == len(r)
		}
		if p[pi] == "**" {
			for k := ri; k <= len(r); k++ {
				if match(k, pi+1) {
					return true
				}
			}
			return false
		}
		if ri >= len(r) {
			return false
		}
		ok, err := path.Match(p[pi], r[ri])
		if err != nil || !ok {
			return false
		}
		return match(ri+1, pi+1)
	}
	return match(0, 0)
}
