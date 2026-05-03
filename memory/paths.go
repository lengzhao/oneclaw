package memory

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// MEMORYMDMaxBytes is the hard cap for MEMORY.md content injected into prompts (stage 6).
const MEMORYMDMaxBytes = 2048

var memoryMonthMarkdown = regexp.MustCompile(`^memory/(\d{4}-\d{2})/([^/]+\.md)$`)

// MonthUTC returns the yyyy-mm segment for t in UTC (memory_extractor month folders).
func MonthUTC(t time.Time) string {
	return t.UTC().Format("2006-01")
}

// RequireWriteUsesCurrentUTCMemoryMonth ensures pathOrRel resolves to memory/<yyyy-mm>/… where yyyy-mm equals MonthUTC(now).
// Write/append tools use this so evolution extracts cannot land in an arbitrary past/future month; reads stay unrestricted.
func RequireWriteUsesCurrentUTCMemoryMonth(pathOrRel string, now time.Time) error {
	rel, err := NormalizeMemoryMonthRel(pathOrRel)
	if err != nil {
		return err
	}
	m := memoryMonthMarkdown.FindStringSubmatch(rel)
	if m == nil {
		return fmt.Errorf("memory: path must be memory/YYYY-MM/name.md")
	}
	want := MonthUTC(now)
	if m[1] != want {
		return fmt.Errorf("memory: write/append must use current UTC month folder memory/%s/... (got month %q)", want, m[1])
	}
	return nil
}

// NormalizeMemoryMonthRel canonicalizes user/tool-provided paths before validation.
// Accepts optional leading "./", case-insensitive "memory/" prefix, or "<yyyy-mm>/<file>.md" with implied "memory/".
func NormalizeMemoryMonthRel(rel string) (string, error) {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" || strings.Contains(rel, "..") {
		return "", fmt.Errorf("memory: invalid relative path")
	}
	rel = strings.TrimPrefix(rel, "./")
	if len(rel) >= 7 && strings.EqualFold(rel[:6], "memory") && rel[6] == '/' {
		rel = "memory/" + rel[7:]
	}
	if !strings.HasPrefix(rel, "memory/") && memoryMonthShort.MatchString(rel) {
		rel = "memory/" + rel
	}
	// Models often copy doc placeholders literally (e.g. memory/YYYY-MM/note.md).
	mm := MonthUTC(time.Now())
	switch {
	case strings.Contains(rel, "YYYY-MM"):
		rel = strings.Replace(rel, "YYYY-MM", mm, 1)
	case strings.Contains(rel, "yyyy-mm"):
		rel = strings.Replace(rel, "yyyy-mm", mm, 1)
	}
	return rel, nil
}

// memoryMonthShort matches yyyy-mm/name.md without the memory/ prefix.
var memoryMonthShort = regexp.MustCompile(`^(\d{4}-\d{2})/([^/]+\.md)$`)

// ResolveMemoryMonthMarkdown maps rel (slash form: memory/yyyy-mm/name.md) under instructionRoot.
func ResolveMemoryMonthMarkdown(instructionRoot, rel string) (abs string, err error) {
	instructionRoot = filepath.Clean(strings.TrimSpace(instructionRoot))
	if instructionRoot == "" || instructionRoot == "." {
		return "", fmt.Errorf("memory: instruction root required")
	}
	rel, err = NormalizeMemoryMonthRel(rel)
	if err != nil {
		return "", err
	}
	m := memoryMonthMarkdown.FindStringSubmatch(rel)
	if m == nil {
		ex := MonthUTC(time.Now())
		return "", fmt.Errorf("memory: path must be memory/YYYY-MM/name.md (example memory/%s/note.md; got %q)", ex, rel)
	}
	if _, err := time.Parse("2006-01", m[1]); err != nil {
		return "", fmt.Errorf("memory: invalid month segment %q", m[1])
	}
	full := filepath.Clean(filepath.Join(instructionRoot, filepath.FromSlash(rel)))
	out, err := filepath.Rel(instructionRoot, full)
	if err != nil || strings.HasPrefix(out, "..") {
		return "", fmt.Errorf("memory: path escapes instruction root")
	}
	if !strings.HasPrefix(filepath.ToSlash(out), "memory/") {
		return "", fmt.Errorf("memory: path must stay under memory/")
	}
	return full, nil
}

// Allowed skill artifact extensions under skills/<skill-id>/ (SKILL.md, scripts, reference docs).
var skillArtifactExtensions = map[string]struct{}{
	".md": {}, ".txt": {}, ".json": {}, ".yaml": {}, ".yml": {},
	".py": {}, ".sh": {}, ".bash": {}, ".zsh": {},
	".js": {}, ".ts": {}, ".mjs": {}, ".cjs": {},
	".html": {}, ".htm": {}, ".css": {}, ".csv": {},
	".rs": {}, ".go": {}, ".toml": {},
}

// ResolveSkillsMarkdown maps rel under userDataRoot/skills/<skill-id>/... with a safe extension (see skillArtifactExtensions).
func ResolveSkillsMarkdown(userDataRoot, rel string) (abs string, err error) {
	userDataRoot = filepath.Clean(strings.TrimSpace(userDataRoot))
	if userDataRoot == "" || userDataRoot == "." {
		return "", fmt.Errorf("skills: user data root required")
	}
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" || strings.Contains(rel, "..") {
		return "", fmt.Errorf("skills: invalid relative path")
	}
	if !strings.HasPrefix(rel, "skills/") {
		return "", fmt.Errorf("skills: path must start with skills/")
	}
	rest := strings.TrimPrefix(rel, "skills/")
	if rest == "" {
		return "", fmt.Errorf("skills: path must be skills/<skill-id>/...")
	}
	firstSlash := strings.Index(rest, "/")
	var skillID string
	var after string
	if firstSlash < 0 {
		return "", fmt.Errorf("skills: path must include a file under skills/<skill-id>/")
	}
	skillID, after = rest[:firstSlash], rest[firstSlash+1:]
	if skillID == "" || after == "" || strings.Contains(after, "..") {
		return "", fmt.Errorf("skills: invalid path under skill folder")
	}
	if !validSkillFolderName(skillID) {
		return "", fmt.Errorf("skills: invalid skill id %q (use [a-z0-9-]+)", skillID)
	}
	base := filepath.Base(filepath.FromSlash(after))
	if base == "" || base == "." || strings.HasPrefix(base, ".") {
		return "", fmt.Errorf("skills: hidden or invalid file name")
	}
	ext := strings.ToLower(filepath.Ext(base))
	if ext == "" {
		return "", fmt.Errorf("skills: file must have an extension")
	}
	if _, ok := skillArtifactExtensions[ext]; !ok {
		return "", fmt.Errorf("skills: extension %q is not allowed for skill artifacts", ext)
	}
	full := filepath.Clean(filepath.Join(userDataRoot, filepath.FromSlash(rel)))
	out, err := filepath.Rel(userDataRoot, full)
	if err != nil || strings.HasPrefix(out, "..") {
		return "", fmt.Errorf("skills: path escapes user data root")
	}
	if !strings.HasPrefix(filepath.ToSlash(out), "skills/") {
		return "", fmt.Errorf("skills: path must stay under skills/")
	}
	return full, nil
}

func validSkillFolderName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		isLower := r >= 'a' && r <= 'z'
		isDigit := r >= '0' && r <= '9'
		isHyphen := r == '-'
		if i == 0 && !isLower && !isDigit {
			return false
		}
		if !isLower && !isDigit && !isHyphen {
			return false
		}
	}
	return true
}
