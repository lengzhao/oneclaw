// Package preturn assembles instruction text and tool allowlists (FR-FLOW-03/04).
package preturn

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/paths"
)

// Bundle is PreTurn output for one agent invocation.
type Bundle struct {
	Instruction   string
	ToolAllowlist []string // empty or nil => allow all registered tools
}

// BuildOpts tweaks PreTurn assembly for sub-agents (phase 4).
type BuildOpts struct {
	// OmitMemory skips MEMORY.md when true (default sub-agent behavior).
	OmitMemory bool
}

// Build constructs system instruction from session AGENT.md, catalog body, MEMORY, and skills index.
func Build(userDataRoot, instructionRoot string, agent *catalog.Agent, budget Budget, opts *BuildOpts) (*Bundle, error) {
	if budget.MemoryMaxRunes == 0 && budget.SkillsMaxRunes == 0 {
		budget = DefaultBudget()
	}
	var parts []string

	if b := readOptionalFile(filepath.Join(instructionRoot, "AGENT.md")); b != "" {
		parts = append(parts, strings.TrimSpace(b))
	}
	if agent != nil && strings.TrimSpace(agent.Body) != "" {
		parts = append(parts, strings.TrimSpace(agent.Body))
	}

	omitMemory := opts != nil && opts.OmitMemory
	if !omitMemory {
		mem := readOptionalFile(filepath.Join(instructionRoot, "MEMORY.md"))
		if mem != "" {
			mem = truncateRunes(mem, budget.MemoryMaxRunes)
			parts = append(parts, "## MEMORY snapshot\n"+mem)
		}
	}

	skillsRoot := filepath.Join(paths.CatalogRoot(userDataRoot), "skills")
	digest, _ := skillsDigest(skillsRoot, budget.SkillsMaxRunes)
	if digest != "" {
		parts = append(parts, "## Skills index\n"+digest)
	}

	instruction := strings.Join(parts, "\n\n---\n\n")
	if strings.TrimSpace(instruction) == "" {
		instruction = "You are a helpful assistant."
	}

	var allow []string
	if agent != nil && len(agent.Tools) > 0 {
		allow = append([]string(nil), agent.Tools...)
	}

	return &Bundle{
		Instruction:   instruction,
		ToolAllowlist: allow,
	}, nil
}

func readOptionalFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func truncateRunes(s string, max int) string {
	if max <= 0 || len([]rune(s)) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "\n…(truncated)"
}

func skillsDigest(skillsRoot string, maxRunes int) (string, error) {
	if _, err := os.Stat(skillsRoot); err != nil {
		return "", nil
	}
	var lines []string
	_ = filepath.WalkDir(skillsRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if strings.EqualFold(d.Name(), "SKILL.md") {
			b, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			title := path
			if i := strings.Index(string(b), "\n"); i > 0 {
				line := strings.TrimSpace(string(b[:i]))
				if strings.HasPrefix(line, "#") {
					title = strings.TrimSpace(strings.TrimPrefix(line, "#"))
				}
			}
			lines = append(lines, "- "+title+" ("+path+")")
		}
		return nil
	})
	if len(lines) == 0 {
		return "", nil
	}
	out := strings.Join(lines, "\n")
	return truncateRunes(out, maxRunes), nil
}
