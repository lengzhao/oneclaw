package skills

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/lengzhao/oneclaw/workspace"
)

// skillFileName is the only supported entry inside each skill directory.
const skillFileName = "SKILL.md"

func userSkillsRoot(home string) string {
	return filepath.Join(home, workspace.DotDir, "skills")
}

func projectSkillsRoot(cwd string, workspaceFlat bool, instructionRoot string) string {
	if strings.TrimSpace(instructionRoot) != "" {
		return filepath.Join(filepath.Clean(instructionRoot), "skills")
	}
	return workspace.JoinSessionWorkspace(cwd, workspaceFlat, "skills")
}

func mergeSkillsFromRoot(byName map[string]Skill, root string) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, ent := range entries {
		if !ent.IsDir() {
			continue
		}
		name := ent.Name()
		if name == "" || name == "." || name == ".." {
			continue
		}
		skillDir := filepath.Join(root, name)
		mdPath := filepath.Join(skillDir, skillFileName)
		st, err := os.Stat(mdPath)
		if err != nil || st.IsDir() {
			continue
		}
		raw, err := os.ReadFile(mdPath)
		if err != nil {
			continue
		}
		fm, _ := ParseFrontmatter(string(raw))
		absRoot, _ := filepath.Abs(skillDir)
		absFile, _ := filepath.Abs(mdPath)
		byName[name] = Skill{
			Name:        name,
			RootDir:     absRoot,
			FilePath:    absFile,
			Description: strings.TrimSpace(fm.Description),
			WhenToUse:   strings.TrimSpace(fm.WhenToUse),
		}
	}
}

// LoadAll returns skills from the user skill catalog, then the active session/project skill catalog.
// Invalid YAML frontmatter skips that file (best-effort).
func LoadAll(cwd, home string, workspaceFlat bool, instructionRoot string) []Skill {
	byName := make(map[string]Skill)
	mergeSkillsFromRoot(byName, userSkillsRoot(home))
	mergeSkillsFromRoot(byName, projectSkillsRoot(cwd, workspaceFlat, instructionRoot))
	out := make([]Skill, 0, len(byName))
	for _, s := range byName {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Lookup returns a skill by canonical name, or false.
func Lookup(cwd, home, name string, workspaceFlat bool, instructionRoot string) (Skill, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Skill{}, false
	}
	// Strip leading slash (slash-command style).
	name = strings.TrimPrefix(name, "/")
	all := LoadAll(cwd, home, workspaceFlat, instructionRoot)
	for _, s := range all {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}
