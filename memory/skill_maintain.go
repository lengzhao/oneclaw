package memory

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	lzmodel "github.com/lengzhao/memory/model"
	lzservice "github.com/lengzhao/memory/service"
)

const (
	skillMarkdownFile    = "SKILL.md"
	maxSkillMarkdownBytes = 128 * 1024
)

// ExtractNamespaceSkill is the memory extract namespace for procedural SKILL.md bodies (not persisted to agent_memory.sqlite).
const ExtractNamespaceSkill = lzmodel.NamespaceType("skill")

var skillDirNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

const skillExtractSystemSuffix = `

SKILL EXTRACTION (namespace "skill"):
- You MAY emit zero or more memories with namespace exactly "skill" when the dialog contains a **stable, repeatable procedure** worth reusing (clear steps, tool patterns, verification, pitfalls).
- Each skill memory: **title** MUST be a single skill directory name — letters, digits, dot, underscore, hyphen only; must match ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ (no slashes or paths).
- **content** MUST be the **full** SKILL.md text: YAML frontmatter (at least name + description; optional when_to_use) followed by markdown sections such as When to Use, Procedure, Pitfalls, Verification.
- Prefer **skill** for runnable runbooks; use **knowledge** for one-off facts. Omit **skill** when detail is too thin.
`

func skillAugmentedExtractionPrompt() *lzmodel.ExtractionPrompt {
	base := lzservice.BuiltinExtractionPrompt()
	out := base
	out.ID = base.ID + "-skill"
	out.SystemPrompt = base.SystemPrompt + skillExtractSystemSuffix
	oldEnum := `"namespace":{"enum":["transient","profile","action","knowledge"]}`
	newEnum := `"namespace":{"enum":["transient","profile","action","knowledge","skill"]}`
	if !strings.Contains(out.JSONSchema, oldEnum) {
		slog.Warn("memory.skill_extract.schema_patch_missing_enum", "prompt_id", out.ID)
	} else {
		out.JSONSchema = strings.Replace(out.JSONSchema, oldEnum, newEnum, 1)
	}
	return &out
}

// PostTurnSkillMaintainTrigger is true when this turn should run skill-augmented extraction (heavy tool use or explicit skill load).
func PostTurnSkillMaintainTrigger(turn *PostTurnInput) bool {
	if turn == nil {
		return false
	}
	if len(turn.Tools) >= 5 {
		return true
	}
	for _, e := range turn.Tools {
		if strings.TrimSpace(e.Name) == "invoke_skill" {
			return true
		}
	}
	return false
}

func validateSkillDirectoryName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("invalid skill directory name: empty")
	}
	if filepath.Base(name) != name || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid skill directory name: %q", name)
	}
	if !skillDirNamePattern.MatchString(name) {
		return "", fmt.Errorf("invalid skill directory name: %q", name)
	}
	return name, nil
}

// writeSkillMemoryToDisk writes one extracted skill memory under layout.DotOrDataRoot()/skills/<title>/SKILL.md.
func writeSkillMemoryToDisk(layout Layout, m lzservice.ExtractedMemory) error {
	name, err := validateSkillDirectoryName(m.Title)
	if err != nil {
		return err
	}
	raw := m.Content
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("skill write: empty content")
	}
	if len(raw) > maxSkillMarkdownBytes {
		return fmt.Errorf("skill write: content too large")
	}
	if !utf8.ValidString(raw) {
		return fmt.Errorf("skill write: invalid utf-8")
	}
	root := filepath.Clean(filepath.Join(layout.DotOrDataRoot(), "skills"))
	abs := filepath.Join(root, name, skillMarkdownFile)
	if rel, err := filepath.Rel(root, abs); err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("skill write: path outside skills root")
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(abs, []byte(raw), 0o644); err != nil {
		return err
	}
	AppendMemoryAudit(layout, abs, "skill_extract_write", []byte(raw))
	slog.Info("memory.skill_extract.wrote", "path", abs, "title", name)
	return nil
}

func skillExtractPostHook(layout Layout) func(context.Context, []lzservice.ExtractedMemory) ([]lzservice.ExtractedMemory, error) {
	return func(_ context.Context, mem []lzservice.ExtractedMemory) ([]lzservice.ExtractedMemory, error) {
		if len(mem) == 0 {
			return mem, nil
		}
		out := make([]lzservice.ExtractedMemory, 0, len(mem))
		for _, m := range mem {
			if m.Namespace == ExtractNamespaceSkill {
				if err := writeSkillMemoryToDisk(layout, m); err != nil {
					slog.Warn("memory.skill_extract.write_failed",
						"title", strings.TrimSpace(m.Title),
						"err", err)
				}
				continue
			}
			out = append(out, m)
		}
		return out, nil
	}
}
