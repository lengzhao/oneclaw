package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

const maxBehaviorPolicyBytes = 128 * 1024

const skillEntryFile = "SKILL.md"

var ruleNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// WriteBehaviorPolicyTool writes scoped project files under the session cwd only: rules, skills, AGENT.md, or project MEMORY.md (no user-global paths).
type WriteBehaviorPolicyTool struct{}

func (WriteBehaviorPolicyTool) Name() string          { return "write_behavior_policy" }
func (WriteBehaviorPolicyTool) ConcurrencySafe() bool { return false }

func (WriteBehaviorPolicyTool) Description() string {
	return "Write scoped project files under the **current working directory** only: " +
		"`<cwd>/.oneclaw/rules/*.md`, `<cwd>/.oneclaw/AGENT.md`, `<cwd>/.oneclaw/skills/<name>/SKILL.md`, `<cwd>/.oneclaw/memory/MEMORY.md`. " +
		"Targets: `rule`, `skill`, `agent_md`, `memory` (rule_name for rule/skill only; omit for agent_md and memory). " +
		"Target `memory` replaces the **entire** project MEMORY.md (**standing rules only**); episodic facts belong in `.oneclaw/memory/YYYY-MM-DD.md` (maintenance merges there). Use `memory` only when intentionally rewriting rules."
}

func (WriteBehaviorPolicyTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"target": map[string]any{
			"type":        "string",
			"description": "rule | skill | agent_md | memory",
			"enum":        []string{"rule", "skill", "agent_md", "memory"},
		},
		"rule_name": map[string]any{
			"type": "string",
			"description": "For `rule`: file name such as editing.md. For `skill`: skill directory name (e.g. my-skill). " +
				"Empty for `agent_md` or `memory`.",
		},
		"content": map[string]any{
			"type":        "string",
			"description": "Full markdown body to write (replaces existing file)",
		},
	}, []string{"target", "content"})
}

func behaviorPolicyWriteDisabled() bool {
	return rtopts.Current().DisableBehaviorPolicyWrite
}

func validatedSkillStem(ruleName string) (string, error) {
	name := strings.TrimSpace(ruleName)
	if name == "" {
		return "", fmt.Errorf("rule_name is required")
	}
	base := filepath.Base(name)
	if base != name || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid rule_name")
	}
	if !ruleNamePattern.MatchString(name) {
		return "", fmt.Errorf("rule_name has invalid characters")
	}
	return name, nil
}

func resolveBehaviorPolicyPath(cwd, target, ruleName string) (string, error) {
	switch target {
	case "rule":
		name := strings.TrimSpace(ruleName)
		if name == "" {
			return "", fmt.Errorf("rule_name is required for rule")
		}
		base := filepath.Base(name)
		if base != name || strings.Contains(name, "..") {
			return "", fmt.Errorf("invalid rule_name")
		}
		stem := name
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			stem = name[:len(name)-3]
		}
		if stem == "" || !ruleNamePattern.MatchString(stem) {
			return "", fmt.Errorf("rule_name has invalid characters")
		}
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			name += ".md"
		}
		dir := filepath.Join(cwd, memory.DotDir, "rules")
		return filepath.Join(dir, name), nil
	case "skill":
		stem, err := validatedSkillStem(ruleName)
		if err != nil {
			return "", fmt.Errorf("skill: %w", err)
		}
		return filepath.Join(cwd, memory.DotDir, "skills", stem, skillEntryFile), nil
	case "agent_md":
		return filepath.Join(cwd, memory.DotDir, memory.AgentInstructionsFile), nil
	case "memory":
		return memory.ProjectMemoryMdPath(cwd), nil
	default:
		return "", fmt.Errorf("unknown target %q", target)
	}
}

func (WriteBehaviorPolicyTool) Execute(_ context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	if behaviorPolicyWriteDisabled() {
		return "", fmt.Errorf("write_behavior_policy is disabled (features.disable_behavior_policy_write in config)")
	}
	var in struct {
		Target   string `json:"target"`
		RuleName string `json:"rule_name"`
		Content  string `json:"content"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	if tctx == nil || strings.TrimSpace(tctx.CWD) == "" {
		return "", fmt.Errorf("missing session cwd")
	}
	content := in.Content
	if len(content) > maxBehaviorPolicyBytes {
		return "", fmt.Errorf("content exceeds max size (%d bytes)", maxBehaviorPolicyBytes)
	}
	if !utf8.ValidString(content) {
		return "", fmt.Errorf("content must be valid UTF-8")
	}
	target := strings.TrimSpace(in.Target)
	if (target == "agent_md" || target == "memory") && strings.TrimSpace(in.RuleName) != "" {
		return "", fmt.Errorf("rule_name must be empty for target %q", target)
	}
	home := tctx.HomeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	abs, err := resolveBehaviorPolicyPath(tctx.CWD, target, in.RuleName)
	if err != nil {
		return "", err
	}
	lay := memory.DefaultLayout(tctx.CWD, home)
	if err := validatePolicyPath(lay, abs, target); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	b := []byte(content)
	if err := os.WriteFile(abs, b, 0o644); err != nil {
		return "", err
	}
	memory.AppendMemoryAudit(lay, abs, "write_behavior_policy", b)
	return "ok: wrote " + abs, nil
}

func validatePolicyPath(lay memory.Layout, abs, target string) error {
	abs = filepath.Clean(abs)
	switch target {
	case "rule":
		rulesDir := filepath.Clean(filepath.Join(lay.CWD, memory.DotDir, "rules"))
		if !memory.PathUnderRoot(abs, rulesDir) {
			return fmt.Errorf("internal error: rule path outside .oneclaw/rules")
		}
	case "skill":
		skillsRoot := filepath.Clean(filepath.Join(lay.CWD, memory.DotDir, "skills"))
		if !memory.PathUnderRoot(abs, skillsRoot) || filepath.Base(abs) != skillEntryFile {
			return fmt.Errorf("internal error: skill path outside .oneclaw/skills or wrong file")
		}
		rel, err := filepath.Rel(skillsRoot, filepath.Dir(abs))
		if err != nil || rel == "." || strings.Contains(rel, "..") || strings.Contains(rel, string(filepath.Separator)) {
			return fmt.Errorf("internal error: skill must be <cwd>/.oneclaw/skills/<name>/SKILL.md")
		}
	case "agent_md":
		want := filepath.Clean(filepath.Join(lay.CWD, memory.DotDir, memory.AgentInstructionsFile))
		if abs != want {
			return fmt.Errorf("internal error: agent path mismatch")
		}
	case "memory":
		want := filepath.Clean(memory.ProjectMemoryMdPath(lay.CWD))
		if abs != want {
			return fmt.Errorf("internal error: memory entrypoint path mismatch")
		}
	default:
		return fmt.Errorf("unknown target %q", target)
	}
	return nil
}
