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

// WriteBehaviorPolicyTool writes scoped project files: rules, skills, AGENT.md, project MEMORY.md, plus arbitrary agent memory roots.
type WriteBehaviorPolicyTool struct{}

func (WriteBehaviorPolicyTool) Name() string          { return "write_behavior_policy" }
func (WriteBehaviorPolicyTool) ConcurrencySafe() bool { return false }

func (WriteBehaviorPolicyTool) Description() string {
	return "Write scoped policy/memory files: " +
		"session rules, session `AGENT.md`, session skills, project `MEMORY.md`, " +
		"plus **agent memory** (`agent_memory` with `agent_type=<name>` and optional `scope=public|local`; writes under `agent-memory/<agent_type>`). " +
		"Targets: `rule`, `skill`, `agent_md`, `memory`, `agent_memory`. " +
		"`rule_name` is for `rule`/`skill` file or dir names; for `agent_memory` it is an optional **relative** path under that root (default `MEMORY.md`). " +
		"Target `memory` replaces the **entire** project rules MEMORY.md; episodic facts belong in `<project>/YYYY-MM-DD.md` (maintenance merges there)."
}

func (WriteBehaviorPolicyTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"target": map[string]any{
			"type": "string",
			"description": "rule | skill | agent_md | memory | agent_memory",
			"enum": []string{
				"rule", "skill", "agent_md", "memory", "agent_memory",
			},
		},
		"rule_name": map[string]any{
			"type": "string",
			"description": "For `rule`: file name such as editing.md. For `skill`: skill directory name (e.g. my-skill). " +
				"Empty for `agent_md` or `memory`. For `agent_memory`: optional relative file path under that root (default MEMORY.md).",
		},
		"scope": map[string]any{
			"type":        "string",
			"description": "For `agent_memory` only: optional `public` (user-wide under memory base) or `local` (workspace/session-local). Omit when the tool can infer it (same root or isolated session -> local).",
			"enum":        []string{"public", "local"},
		},
		"agent_type": map[string]any{
			"type":        "string",
			"description": "For `agent_memory` only: agent type name, e.g. `default`, `explore`, `reviewer`.",
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

func validatedAgentType(agentType string) (string, error) {
	name := strings.TrimSpace(agentType)
	if name == "" {
		return "", fmt.Errorf("agent_type is required for agent_memory")
	}
	base := filepath.Base(name)
	if base != name || strings.Contains(name, "..") {
		return "", fmt.Errorf("invalid agent_type")
	}
	if !ruleNamePattern.MatchString(name) {
		return "", fmt.Errorf("agent_type has invalid characters")
	}
	return name, nil
}

func inferAgentMemoryScope(lay memory.Layout, hostDataRoot, scope string) (string, error) {
	scope = strings.TrimSpace(scope)
	if scope != "" {
		if scope != "public" && scope != "local" {
			return "", fmt.Errorf("scope must be public or local")
		}
		return scope, nil
	}
	if len(lay.AgentDefault) < 2 {
		return "", fmt.Errorf("agent_memory: layout missing public/local roots")
	}
	publicRoot := filepath.Clean(filepath.Dir(lay.AgentDefault[0]))
	localRoot := filepath.Clean(filepath.Dir(lay.AgentDefault[1]))
	if publicRoot == localRoot {
		return "public", nil
	}
	hr := strings.TrimSpace(hostDataRoot)
	ir := strings.TrimSpace(lay.InstructionRoot)
	if hr != "" && ir != "" && filepath.Clean(hr) != filepath.Clean(ir) {
		return "local", nil
	}
	return "", fmt.Errorf("scope is required for target %q when public/local roots differ", "agent_memory")
}

func resolveBehaviorPolicyPath(cwd string, workspaceFlat bool, instructionRoot, target, ruleName string) (string, error) {
	ir := strings.TrimSpace(instructionRoot)
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
		var dir string
		if ir != "" {
			dir = filepath.Join(filepath.Clean(ir), "rules")
		} else {
			dir = memory.JoinSessionWorkspace(cwd, workspaceFlat, "rules")
		}
		return filepath.Join(dir, name), nil
	case "skill":
		stem, err := validatedSkillStem(ruleName)
		if err != nil {
			return "", fmt.Errorf("skill: %w", err)
		}
		if ir != "" {
			return filepath.Join(filepath.Clean(ir), "skills", stem, skillEntryFile), nil
		}
		return memory.JoinSessionWorkspace(cwd, workspaceFlat, "skills", stem, skillEntryFile), nil
	case "agent_md":
		if ir != "" {
			return filepath.Join(filepath.Clean(ir), memory.AgentInstructionsFile), nil
		}
		return memory.JoinSessionWorkspace(cwd, workspaceFlat, memory.AgentInstructionsFile), nil
	case "memory":
		if ir != "" {
			return filepath.Join(filepath.Clean(ir), "MEMORY.md"), nil
		}
		return memory.JoinSessionWorkspace(cwd, workspaceFlat, "memory", "MEMORY.md"), nil
	default:
		return "", fmt.Errorf("unknown target %q", target)
	}
}

// behaviorPolicyRelativePath returns a safe relative path under a memory root; empty ruleName means MEMORY.md.
func behaviorPolicyRelativePath(ruleName string) (string, error) {
	s := strings.TrimSpace(ruleName)
	if s == "" {
		return "MEMORY.md", nil
	}
	if filepath.IsAbs(s) {
		return "", fmt.Errorf("rule_name must be a relative path")
	}
	clean := filepath.Clean(s)
	if clean == "." || strings.HasPrefix(clean, "..") {
		return "", fmt.Errorf("invalid rule_name")
	}
	if len(clean) >= 2 && clean[1] == ':' {
		return "", fmt.Errorf("invalid rule_name")
	}
	return clean, nil
}

func resolveBehaviorPolicyPathWithLayout(lay memory.Layout, cwd string, workspaceFlat bool, instructionRoot, hostDataRoot, target, ruleName, scope, agentType string) (string, error) {
	switch target {
	case "agent_memory":
		rel, err := behaviorPolicyRelativePath(ruleName)
		if err != nil {
			return "", err
		}
		scope, err = inferAgentMemoryScope(lay, hostDataRoot, scope)
		if err != nil {
			return "", err
		}
		name, err := validatedAgentType(agentType)
		if err != nil {
			return "", err
		}
		var rootBase string
		switch scope {
		case "public":
			if len(lay.AgentDefault) < 1 {
				return "", fmt.Errorf("agent_memory: layout has no public root")
			}
			rootBase = filepath.Dir(lay.AgentDefault[0])
		case "local":
			if len(lay.AgentDefault) < 2 {
				return "", fmt.Errorf("agent_memory: layout has no local root")
			}
			rootBase = filepath.Dir(lay.AgentDefault[1])
		default:
			return "", fmt.Errorf("scope is required for target %q and must be public or local", target)
		}
		rootBase = strings.TrimSpace(rootBase)
		if rootBase == "" || rootBase == "." {
			return "", fmt.Errorf("%s: empty %s root", target, scope)
		}
		root := filepath.Join(filepath.Clean(rootBase), name)
		return filepath.Join(root, rel), nil
	default:
		return resolveBehaviorPolicyPath(cwd, workspaceFlat, instructionRoot, target, ruleName)
	}
}

func (WriteBehaviorPolicyTool) Execute(_ context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	if behaviorPolicyWriteDisabled() {
		return "", fmt.Errorf("write_behavior_policy is disabled (features.disable_behavior_policy_write in config)")
	}
	var in struct {
		Target    string `json:"target"`
		RuleName  string `json:"rule_name"`
		Scope     string `json:"scope"`
		AgentType string `json:"agent_type"`
		Content   string `json:"content"`
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
	if target != "agent_memory" {
		if strings.TrimSpace(in.Scope) != "" {
			return "", fmt.Errorf("scope must be empty for target %q", target)
		}
		if strings.TrimSpace(in.AgentType) != "" {
			return "", fmt.Errorf("agent_type must be empty for target %q", target)
		}
	}
	home := tctx.HomeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	lay := memory.LayoutForIMWorkspace(tctx.CWD, home, tctx.HostDataRoot, tctx.WorkspaceFlat, tctx.InstructionRoot)
	abs, err := resolveBehaviorPolicyPathWithLayout(lay, tctx.CWD, tctx.WorkspaceFlat, tctx.InstructionRoot, tctx.HostDataRoot, target, in.RuleName, in.Scope, in.AgentType)
	if err != nil {
		return "", err
	}
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
		rulesDir := filepath.Clean(filepath.Join(lay.DotOrDataRoot(), "rules"))
		if !memory.PathUnderRoot(abs, rulesDir) {
			return fmt.Errorf("internal error: rule path outside rules dir")
		}
	case "skill":
		skillsRoot := filepath.Clean(filepath.Join(lay.DotOrDataRoot(), "skills"))
		if !memory.PathUnderRoot(abs, skillsRoot) || filepath.Base(abs) != skillEntryFile {
			return fmt.Errorf("internal error: skill path outside skills root or wrong file")
		}
		rel, err := filepath.Rel(skillsRoot, filepath.Dir(abs))
		if err != nil || rel == "." || strings.Contains(rel, "..") || strings.Contains(rel, string(filepath.Separator)) {
			return fmt.Errorf("internal error: skill must be <workspace>/skills/<name>/SKILL.md")
		}
	case "agent_md":
		want := filepath.Clean(filepath.Join(lay.DotOrDataRoot(), memory.AgentInstructionsFile))
		if abs != want {
			return fmt.Errorf("internal error: agent path mismatch")
		}
	case "memory":
		want := filepath.Clean(filepath.Join(lay.Project, lay.EntrypointName))
		if lay.InstructionRoot != "" {
			want = filepath.Clean(filepath.Join(lay.InstructionRoot, lay.EntrypointName))
		}
		if abs != want {
			return fmt.Errorf("internal error: memory entrypoint path mismatch")
		}
	case "agent_memory":
		name, err := validatedAgentType(targetAgentType(abs, lay))
		if err != nil {
			return err
		}
		_ = name
		if len(lay.AgentDefault) < 1 {
			return fmt.Errorf("internal error: missing agent_memory public root")
		}
		publicRoot := filepath.Clean(filepath.Dir(lay.AgentDefault[0]))
		if memory.PathUnderRoot(abs, publicRoot) {
			return nil
		}
		if len(lay.AgentDefault) < 2 {
			return fmt.Errorf("internal error: missing agent_memory local root")
		}
		localRoot := filepath.Clean(filepath.Dir(lay.AgentDefault[1]))
		if memory.PathUnderRoot(abs, localRoot) {
			return nil
		}
		return fmt.Errorf("internal error: agent_memory path outside roots")
	default:
		return fmt.Errorf("unknown target %q", target)
	}
	return nil
}

func targetAgentType(abs string, lay memory.Layout) string {
	roots := []string{}
	if len(lay.AgentDefault) > 0 {
		roots = append(roots, filepath.Clean(filepath.Dir(lay.AgentDefault[0])))
	}
	if len(lay.AgentDefault) > 1 {
		roots = append(roots, filepath.Clean(filepath.Dir(lay.AgentDefault[1])))
	}
	for _, root := range roots {
		if !memory.PathUnderRoot(abs, root) {
			continue
		}
		rel, err := filepath.Rel(root, abs)
		if err != nil {
			continue
		}
		parts := strings.Split(rel, string(filepath.Separator))
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}
