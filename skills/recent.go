package skills

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/workspace"
	"github.com/lengzhao/oneclaw/rtopts"
)

// RecentEntry is one row in skills-recent.json.
type RecentEntry struct {
	Name       string    `json:"name"`
	LastUsedAt time.Time `json:"last_used_at"`
	UseCount   int       `json:"use_count"`
}

// RecentFile is the on-disk JSON shape.
type RecentFile struct {
	Version int           `json:"version"`
	Entries []RecentEntry `json:"entries"`
}

// RecentFilePath returns where we persist recent skill usage (project-local by default).
func RecentFilePath(cwd string, workspaceFlat bool, instructionRoot string) string {
	if p := strings.TrimSpace(rtopts.Current().SkillsRecent); p != "" {
		return filepath.Clean(p)
	}
	return workspace.JoinSessionWorkspaceWithInstruction(cwd, instructionRoot, workspaceFlat, "skills-recent.json")
}

// LoadRecent reads the recent-usage file. Missing or invalid file yields empty Version 1.
func LoadRecent(path string) (RecentFile, error) {
	var out RecentFile
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return RecentFile{Version: 1, Entries: nil}, nil
		}
		return out, err
	}
	if err := json.Unmarshal(b, &out); err != nil {
		return RecentFile{Version: 1, Entries: nil}, nil
	}
	if out.Version == 0 {
		out.Version = 1
	}
	return out, nil
}

// NamesInOrder returns entry names in file order (expected newest-first after RecordUse).
func (r RecentFile) NamesInOrder() []string {
	var names []string
	for _, e := range r.Entries {
		n := strings.TrimSpace(e.Name)
		if n != "" {
			names = append(names, n)
		}
	}
	return names
}

// RecordUse moves name to the front, bumps use count, trims to MaxRecentEntries, and saves atomically.
func RecordUse(cwd, name string, workspaceFlat bool, instructionRoot string) error {
	name = strings.TrimSpace(strings.TrimPrefix(name, "/"))
	if name == "" {
		return nil
	}
	path := RecentFilePath(cwd, workspaceFlat, instructionRoot)
	rec, err := LoadRecent(path)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var rest []RecentEntry
	for _, e := range rec.Entries {
		if strings.TrimSpace(e.Name) == name {
			continue
		}
		rest = append(rest, e)
	}
	var prevCount int
	for _, e := range rec.Entries {
		if strings.TrimSpace(e.Name) == name {
			prevCount = e.UseCount
			break
		}
	}
	head := RecentEntry{Name: name, LastUsedAt: now, UseCount: prevCount + 1}
	merged := append([]RecentEntry{head}, rest...)
	if len(merged) > MaxRecentEntries {
		merged = merged[:MaxRecentEntries]
	}
	out := RecentFile{Version: 1, Entries: merged}
	return atomicWriteJSON(path, out)
}

func atomicWriteJSON(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	tmp, err := os.CreateTemp(dir, ".skills-recent-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}
