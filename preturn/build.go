// Package preturn assembles instruction text and tool allowlists (FR-FLOW-03/04).
package preturn

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/memory"
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
	// OmitFileBackedPromptBlocks skips AGENT.md, MEMORY.md, skills digest, and referenced-skill injection when true.
	// Main turns combine workflow-filled template vars + agents/<type>.prompt.tmpl at wfexec adk_main instead.
	OmitFileBackedPromptBlocks bool
}

// Build constructs system instruction from session AGENT.md, catalog body, MEMORY, and skills index.
func Build(userDataRoot, instructionRoot string, agent *catalog.Agent, budget Budget, opts *BuildOpts) (*Bundle, error) {
	budget = CoalesceBudget(budget)
	var parts []string

	stripPromptFiles := opts != nil && opts.OmitFileBackedPromptBlocks &&
		(agent == nil || agent.AgentType != "memory_extractor")

	if !stripPromptFiles {
		if b := readOptionalFile(filepath.Join(instructionRoot, "AGENT.md")); b != "" {
			parts = append(parts, strings.TrimSpace(b))
		}
		if agent != nil && len(NormalizeSkillRefs(agent.ReferencedSkillIDs)) > 0 {
			if s := ReferencedSkillsIndexMarkdown(userDataRoot, agent.ReferencedSkillIDs); s != "" {
				parts = append(parts, s)
			}
		}
	}
	if agent != nil && agent.AgentType == "memory_extractor" {
		tree := memoryFolderTreeDigest(instructionRoot, budget.MemoryFolderMaxRunes)
		if tree == "" {
			tree = "(no `memory/` tree yet — write extracts as `memory/<UTC-yyyy-mm>/<name>.md` under the instruction root.)"
		}
		parts = append(parts, "## Memory folder (instruction root)\n\n"+tree)
	}
	if !stripPromptFiles {
		if agent != nil && strings.TrimSpace(agent.Body) != "" {
			parts = append(parts, strings.TrimSpace(agent.Body))
		}
	}

	omitMemory := opts != nil && opts.OmitMemory
	if !omitMemory && !stripPromptFiles {
		memPath := filepath.Join(instructionRoot, "MEMORY.md")
		if raw, err := os.ReadFile(memPath); err == nil && len(raw) > 0 {
			// MEMORY.md: 2048-byte cap is the write-side contract; injection applies the same byte ceiling only.
			// MemoryMaxRunes is reserved for lengzhao/memory (SQLite) recall when wired.
			raw = memory.TruncateMEMORYMDForInjection(raw)
			parts = append(parts, "## MEMORY snapshot\n"+string(raw))
		}
	}

	if !stripPromptFiles {
		skillsRoot := filepath.Join(paths.CatalogRoot(userDataRoot), "skills")
		var catalogSkills []string
		if agent != nil {
			catalogSkills = agent.ReferencedSkillIDs
		}
		digest, err := skillsDigest(skillsRoot, budget, catalogSkills)
		if err != nil {
			return nil, err
		}
		if digest != "" {
			parts = append(parts, "## Skills index\n"+digest)
		}
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
