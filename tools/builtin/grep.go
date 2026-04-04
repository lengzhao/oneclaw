package builtin

import (
	"bufio"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools/pathutil"
)

const grepMaxMatches = 200
const grepMaxLineBytes = 4096

type grepInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

// GrepTool searches for a regex pattern in a single file or all files under a directory (non-recursive one level + recursive option simplified: walk with skip dot dirs).
type GrepTool struct{}

func (GrepTool) Name() string        { return "grep" }
func (GrepTool) ConcurrencySafe() bool { return true }
func (GrepTool) Description() string {
	return "Search for a regular expression in files under the working directory. Path is a file or directory (directory walk skips .git, node_modules)."
}

func (GrepTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"pattern": map[string]any{
			"type":        "string",
			"description": "Go regexp pattern",
		},
		"path": map[string]any{
			"type":        "string",
			"description": "File or directory relative to cwd",
		},
	}, []string{"pattern", "path"})
}

func (GrepTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in grepInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	re, err := regexp.Compile(in.Pattern)
	if err != nil {
		return "", err
	}
	rootAbs, err := pathutil.ResolveForSession(tctx.CWD, tctx.MemoryWriteRoots, in.Path)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	n := 0
	st, err := os.Stat(rootAbs)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		if err := grepFile(ctx, re, rootAbs, &b, &n); err != nil {
			return "", err
		}
		if b.Len() == 0 {
			return "(no matches)", nil
		}
		return b.String(), nil
	}
	err = filepath.WalkDir(rootAbs, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if d.IsDir() {
			base := filepath.Base(path)
			if path != rootAbs && (strings.HasPrefix(base, ".") || base == "node_modules" || base == "vendor") {
				return fs.SkipDir
			}
			return nil
		}
		if n >= grepMaxMatches {
			return fs.SkipAll
		}
		_ = grepFile(ctx, re, path, &b, &n)
		return nil
	})
	if err != nil {
		return b.String(), err
	}
	if b.Len() == 0 {
		return "(no matches)", nil
	}
	if n >= grepMaxMatches {
		b.WriteString("\n[truncated: max matches reached]\n")
	}
	return b.String(), nil
}

func grepFile(ctx context.Context, re *regexp.Regexp, path string, out *strings.Builder, n *int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), grepMaxLineBytes)
	for sc.Scan() {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if *n >= grepMaxMatches {
			break
		}
		txt := sc.Text()
		if re.MatchString(txt) {
			*n++
			out.WriteString(path)
			out.WriteByte(':')
			out.WriteString(strings.TrimSpace(strings.ReplaceAll(txt, "\n", " ")))
			out.WriteByte('\n')
		}
	}
	return sc.Err()
}
