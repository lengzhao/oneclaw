package memory

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

const maxRecallFileBytes = 4_096

// SelectRecall returns markdown snippets from memory dirs relevant to userText, respecting budget and dedupe.
func SelectRecall(layout Layout, userText string, state *RecallState, budget int) (string, *RecallState) {
	if budget <= 0 {
		budget = MaxSurfacedRecallBytes
	}
	var st *RecallState
	if state == nil {
		st = &RecallState{SurfacedPaths: make(map[string]struct{})}
	} else {
		st = state.cloneMaps()
	}

	candidates := listMemoryMarkdownFiles(layout)
	if len(candidates) == 0 {
		return "", st
	}
	terms := tokenizeRecall(userText)
	if len(terms) == 0 {
		return "", st
	}

	type scored struct {
		path  string
		score int
		text  string
	}
	var hits []scored
	for _, p := range candidates {
		if _, dup := st.SurfacedPaths[p]; dup {
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		body := StripYAMLFrontmatter(string(b))
		if len(body) > maxRecallFileBytes {
			body = body[:maxRecallFileBytes] + "\n…"
		}
		low := strings.ToLower(body)
		base := strings.ToLower(filepath.Base(p))
		s := scoreRecall(low, base, terms)
		if s > 0 {
			hits = append(hits, scored{path: p, score: s, text: body})
		}
	}
	// Highest score first
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].score > hits[i].score {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}

	var sb strings.Builder
	remaining := budget - st.SurfacedBytes
	if remaining <= 0 {
		return "", st
	}
	header := "Attachment: relevant_memories\n\n"
	if len(header) > remaining {
		return "", st
	}
	sb.WriteString(header)
	remaining -= len(header)

	for _, h := range hits {
		block := "Memory: " + h.path + "\n" + strings.TrimSpace(h.text) + "\n\n"
		if len(block) > remaining {
			break
		}
		sb.WriteString(block)
		remaining -= len(block)
		st.SurfacedPaths[h.path] = struct{}{}
		st.SurfacedBytes += len(block)
	}
	out := sb.String()
	if out == header {
		return "", st
	}
	st.SurfacedBytes += len(header)
	return strings.TrimRight(out, "\n"), st
}

func listMemoryMarkdownFiles(layout Layout) []string {
	dirs := []string{
		layout.User, layout.Project,
		layout.TeamUser, layout.TeamProject,
	}
	if !AutoMemoryDisabled() {
		dirs = append(dirs, layout.Auto)
	}
	for _, a := range layout.AgentDefault {
		dirs = append(dirs, a)
	}
	seen := make(map[string]struct{})
	var files []string
	for _, root := range dirs {
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".md") {
				return nil
			}
			if _, ok := seen[path]; ok {
				return nil
			}
			seen[path] = struct{}{}
			files = append(files, path)
			return nil
		})
	}
	return files
}

func tokenizeRecall(s string) []string {
	s = strings.ToLower(s)
	var cur strings.Builder
	var out []string
	flush := func() {
		t := cur.String()
		cur.Reset()
		if len(t) > 2 {
			out = append(out, t)
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func scoreRecall(contentLower, baseLower string, terms []string) int {
	score := 0
	for _, t := range terms {
		if strings.Contains(baseLower, t) {
			score += 5
		}
		if strings.Contains(contentLower, t) {
			score += 2
		}
	}
	return score
}
