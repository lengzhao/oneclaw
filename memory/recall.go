package memory

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxRecallScanBytes caps how much of each markdown file is read for scoring and snippet extraction.
const maxRecallScanBytes = 512 * 1024

// recallSnippetPadBytes is how many bytes of context to keep on each side of a match (UTF-8 boundaries).
const recallSnippetPadBytes = 120

// maxSnippetsPerFile limits how many distinct context windows are emitted per path.
const maxSnippetsPerFile = 12

// mergeSnippetGapBytes: expanded match windows separated by at most this gap are merged into one snippet.
const mergeSnippetGapBytes = 24

// maxSnippetLineRunes caps display width of a single snippet line.
const maxSnippetLineRunes = 220

// maxRecallTermCount caps unique query terms after tokenization (long paste guardrail).
const maxRecallTermCount = 384

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
		path     string
		score    int
		body     string
		bodyBase int // byte offset in on-disk file where `body` begins (after BOM/frontmatter)
		full     bool // true if entire file (post-frontmatter) fit in scan limit
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
		fileText := string(b)
		bodyBase := BodyStartByteOffset(fileText)
		if bodyBase > len(fileText) {
			bodyBase = len(fileText)
		}
		raw := fileText[bodyBase:]
		full := len(raw) <= maxRecallScanBytes
		body := raw
		if len(body) > maxRecallScanBytes {
			body = raw[:maxRecallScanBytes]
		}
		low := strings.ToLower(body)
		base := strings.ToLower(filepath.Base(p))
		s := scoreRecall(low, base, terms)
		if s > 0 {
			hits = append(hits, scored{path: p, score: s, body: body, bodyBase: bodyBase, full: full})
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
		block := formatRecallMemoryBlock(h.path, h.body, terms, h.full, h.bodyBase)
		if block == "" {
			continue
		}
		wrapped := "Memory: " + h.path + "\n" + block + "\n\n"
		if len(wrapped) > remaining {
			break
		}
		sb.WriteString(wrapped)
		remaining -= len(wrapped)
		st.SurfacedPaths[h.path] = struct{}{}
		st.SurfacedBytes += len(wrapped)
	}
	out := sb.String()
	if out == header {
		return "", st
	}
	st.SurfacedBytes += len(header)
	return strings.TrimRight(out, "\n"), st
}

type rawMatchSpan struct {
	start, end int // byte offsets into body; end exclusive
}

func formatRecallMemoryBlock(path, body string, terms []string, fullFileScanned bool, bodyBase int) string {
	baseLower := strings.ToLower(filepath.Base(path))
	bodyLower := strings.ToLower(body)
	spans := collectRawMatchSpans(bodyLower, body, terms)
	windows := mergeExpandedMatchWindows(body, spans, recallSnippetPadBytes, mergeSnippetGapBytes)
	if len(windows) > maxSnippetsPerFile {
		windows = windows[:maxSnippetsPerFile]
	}
	var sb strings.Builder
	if !fullFileScanned {
		sb.WriteString("(body truncated after ")
		sb.WriteString(strconv.Itoa(maxRecallScanBytes))
		sb.WriteString(" bytes post-frontmatter; offsets below are UTF-8 byte offsets from start of file)\n")
	}
	if len(windows) == 0 {
		line := filenameOnlyRecallLine(baseLower, terms)
		if line == "" {
			return ""
		}
		sb.WriteString(line)
		return strings.TrimRight(sb.String(), "\n")
	}
	for _, w := range windows {
		snip := body[w.lo:w.hi]
		sb.WriteString("- offset ")
		sb.WriteString(strconv.Itoa(bodyBase + w.matchOffset))
		sb.WriteString(" (file bytes): ")
		sb.WriteString(truncateOneLineSnippet(snip, maxSnippetLineRunes))
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

func filenameOnlyRecallLine(baseLower string, terms []string) string {
	var matched []string
	seen := make(map[string]struct{})
	for _, t := range terms {
		if t == "" {
			continue
		}
		tl := strings.ToLower(t)
		if !strings.Contains(baseLower, tl) {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		matched = append(matched, t)
	}
	if len(matched) == 0 {
		return ""
	}
	return "- filename match: " + strings.Join(matched, ", ") + "\n"
}

func truncateOneLineSnippet(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	if maxRunes <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "…"
}

func collectRawMatchSpans(bodyLower, body string, terms []string) []rawMatchSpan {
	if body == "" {
		return nil
	}
	var spans []rawMatchSpan
	for _, term := range terms {
		if term == "" {
			continue
		}
		tl := strings.ToLower(term)
		tlen := len(tl)
		if tlen == 0 {
			continue
		}
		search := 0
		for search < len(bodyLower) {
			idx := strings.Index(bodyLower[search:], tl)
			if idx < 0 {
				break
			}
			abs := search + idx
			end := abs + tlen
			if end > len(body) {
				break
			}
			spans = append(spans, rawMatchSpan{start: abs, end: end})
			search = abs + 1
		}
	}
	return spans
}

type mergedWindow struct {
	lo, hi       int
	matchOffset  int // smallest raw match start inside this window
}

func mergeExpandedMatchWindows(body string, raw []rawMatchSpan, pad, gap int) []mergedWindow {
	if len(raw) == 0 {
		return nil
	}
	bodyLen := len(body)
	type exp struct {
		lo, hi, m int
	}
	exps := make([]exp, 0, len(raw))
	for _, sp := range raw {
		lo := sp.start - pad
		if lo < 0 {
			lo = 0
		}
		hi := sp.end + pad
		if hi > bodyLen {
			hi = bodyLen
		}
		lo = clipUTF8Lo(body, lo)
		hi = clipUTF8Hi(body, hi)
		if lo >= hi {
			continue
		}
		exps = append(exps, exp{lo: lo, hi: hi, m: sp.start})
	}
	if len(exps) == 0 {
		return nil
	}
	sort.Slice(exps, func(i, j int) bool {
		if exps[i].lo != exps[j].lo {
			return exps[i].lo < exps[j].lo
		}
		return exps[i].hi < exps[j].hi
	})
	var out []mergedWindow
	cur := mergedWindow{lo: exps[0].lo, hi: exps[0].hi, matchOffset: exps[0].m}
	for _, e := range exps[1:] {
		if e.lo <= cur.hi+gap {
			if e.hi > cur.hi {
				cur.hi = e.hi
			}
			if e.m < cur.matchOffset {
				cur.matchOffset = e.m
			}
		} else {
			out = append(out, cur)
			cur = mergedWindow{lo: e.lo, hi: e.hi, matchOffset: e.m}
		}
	}
	out = append(out, cur)
	return out
}

func clipUTF8Lo(body string, lo int) int {
	if lo <= 0 {
		return 0
	}
	if lo >= len(body) {
		return len(body)
	}
	for lo > 0 && !utf8.RuneStart(body[lo]) {
		lo--
	}
	return lo
}

func clipUTF8Hi(body string, hi int) int {
	if hi >= len(body) {
		return len(body)
	}
	for hi < len(body) && !utf8.RuneStart(body[hi]) {
		hi++
	}
	return hi
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
	rootClean := func(r string) string {
		r = filepath.Clean(r)
		if r == "." {
			return r
		}
		return r
	}
	for _, root := range dirs {
		r0 := rootClean(root)
		_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".md") {
				return nil
			}
			rel, err := filepath.Rel(r0, path)
			if err != nil {
				return nil
			}
			// Skip rules entrypoint at each memory root (injected via AgentMdBlock); index all other .md files.
			if !strings.Contains(rel, string(filepath.Separator)) && strings.EqualFold(filepath.Base(path), entrypointName) {
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
	var out []string
	seen := make(map[string]struct{})
	add := func(term string) {
		if term == "" {
			return
		}
		if _, ok := seen[term]; ok {
			return
		}
		if len(out) >= maxRecallTermCount {
			return
		}
		seen[term] = struct{}{}
		out = append(out, term)
	}

	var latin strings.Builder
	var han []rune

	flushLatin := func() {
		if latin.Len() == 0 {
			return
		}
		t := strings.ToLower(latin.String())
		latin.Reset()
		if len(t) > 2 {
			add(t)
		}
	}
	flushHan := func() {
		if len(han) < 2 {
			han = han[:0]
			return
		}
		for i := 0; i < len(han)-1; i++ {
			add(string(han[i : i+2]))
		}
		han = han[:0]
	}

	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r):
			flushLatin()
			han = append(han, r)
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			flushHan()
			latin.WriteRune(r)
		default:
			flushLatin()
			flushHan()
		}
	}
	flushLatin()
	flushHan()
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
