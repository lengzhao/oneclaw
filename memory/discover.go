package memory

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// InstructionKind classifies discovered instruction files.
type InstructionKind string

const (
	KindUser    InstructionKind = "user"
	KindProject InstructionKind = "project"
)

// InstructionChunk is one loaded instructions file with metadata.
type InstructionChunk struct {
	Path    string
	Kind    InstructionKind
	Content string
}

// DiscoverUserAgentMd loads ~/.oneclaw/AGENT.md if present.
func DiscoverUserAgentMd(home string) []InstructionChunk {
	p := filepath.Join(MemoryBaseDir(home), AgentInstructionsFile)
	return loadInstructionPath(p, KindUser)
}

// DiscoverUserRules loads ~/.oneclaw/rules/**/*.md (non-conditional simplified: all md).
func DiscoverUserRules(home string) []InstructionChunk {
	dir := filepath.Join(MemoryBaseDir(home), "rules")
	return walkRulesMD(dir, KindUser)
}

// DiscoverProjectInstructions walks from filesystem root toward cwd (outer roots first, cwd last).
func DiscoverProjectInstructions(cwd string) []InstructionChunk {
	chain := walkUpDirs(cwd)
	var out []InstructionChunk
	for i := len(chain) - 1; i >= 0; i-- {
		dir := chain[i]
		out = append(out, loadInstructionPath(filepath.Join(dir, DotDir, AgentInstructionsFile), KindProject)...)
		out = append(out, walkRulesMD(filepath.Join(dir, DotDir, "rules"), KindProject)...)
	}
	return out
}

func walkUpDirs(cwd string) []string {
	var dirs []string
	d := filepath.Clean(cwd)
	for {
		dirs = append(dirs, d)
		parent := filepath.Dir(d)
		if parent == d {
			break
		}
		d = parent
	}
	return dirs
}

func loadInstructionPath(abs string, kind InstructionKind) []InstructionChunk {
	raw, err := os.ReadFile(abs)
	if err != nil {
		return nil
	}
	return []InstructionChunk{{Path: abs, Kind: kind, Content: string(raw)}}
}

func walkRulesMD(rulesDir string, kind InstructionKind) []InstructionChunk {
	var out []InstructionChunk
	_ = filepath.WalkDir(rulesDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.EqualFold(filepath.Ext(path), ".md") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		out = append(out, InstructionChunk{Path: path, Kind: kind, Content: string(raw)})
		return nil
	})
	return out
}
