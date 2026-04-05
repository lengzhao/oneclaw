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
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

const maxBehaviorPolicyBytes = 128 * 1024

var ruleNamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)

// WriteBehaviorPolicyTool writes durable behavior rules to `.oneclaw/rules/*.md` or canonical AGENT.md paths only.
type WriteBehaviorPolicyTool struct{}

func (WriteBehaviorPolicyTool) Name() string          { return "write_behavior_policy" }
func (WriteBehaviorPolicyTool) ConcurrencySafe() bool { return false }

func (WriteBehaviorPolicyTool) Description() string {
	return "Write behavior rules or agent instructions to allowed locations only: " +
		"a markdown file under `<cwd>/.oneclaw/rules/`, or project `AGENT.md`, or `<cwd>/.oneclaw/AGENT.md`, " +
		"or user `~/.oneclaw/AGENT.md`. Use for persistent policies you want injected on future turns. " +
		"Targets: `project_rule` (requires rule_name, .md), `project_agent_md`, `dot_oneclaw_agent_md`, `user_agent_md`."
}

func (WriteBehaviorPolicyTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"target": map[string]any{
			"type":        "string",
			"description": "project_rule | project_agent_md | dot_oneclaw_agent_md | user_agent_md",
			"enum":        []string{"project_rule", "project_agent_md", "dot_oneclaw_agent_md", "user_agent_md"},
		},
		"rule_name": map[string]any{
			"type":        "string",
			"description": "For project_rule only: file name such as editing.md (letters, digits, ._-; .md added if missing)",
		},
		"content": map[string]any{
			"type":        "string",
			"description": "Full markdown body to write (replaces existing file)",
		},
	}, []string{"target", "content"})
}

func behaviorPolicyWriteDisabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_BEHAVIOR_POLICY_WRITE"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

func resolveBehaviorPolicyPath(cwd, home, target, ruleName string) (string, error) {
	switch target {
	case "project_rule":
		name := strings.TrimSpace(ruleName)
		if name == "" {
			return "", fmt.Errorf("rule_name is required for project_rule")
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
	case "project_agent_md":
		return filepath.Join(cwd, memory.AgentInstructionsFile), nil
	case "dot_oneclaw_agent_md":
		return filepath.Join(cwd, memory.DotDir, memory.AgentInstructionsFile), nil
	case "user_agent_md":
		if strings.TrimSpace(home) == "" {
			return "", fmt.Errorf("user home directory is not available")
		}
		return filepath.Join(memory.MemoryBaseDir(home), memory.AgentInstructionsFile), nil
	default:
		return "", fmt.Errorf("unknown target %q", target)
	}
}

func (WriteBehaviorPolicyTool) Execute(_ context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	if behaviorPolicyWriteDisabled() {
		return "", fmt.Errorf("write_behavior_policy is disabled (ONCLAW_DISABLE_BEHAVIOR_POLICY_WRITE)")
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
	home := tctx.HomeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	abs, err := resolveBehaviorPolicyPath(tctx.CWD, home, strings.TrimSpace(in.Target), in.RuleName)
	if err != nil {
		return "", err
	}
	lay := memory.DefaultLayout(tctx.CWD, home)
	if err := validatePolicyPath(lay, abs, strings.TrimSpace(in.Target)); err != nil {
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
	case "project_rule":
		rulesDir := filepath.Clean(filepath.Join(lay.CWD, memory.DotDir, "rules"))
		if !memory.PathUnderRoot(abs, rulesDir) {
			return fmt.Errorf("internal error: rule path outside .oneclaw/rules")
		}
	case "project_agent_md", "dot_oneclaw_agent_md", "user_agent_md":
		if !lay.IsBehaviorPolicyFile(abs) {
			return fmt.Errorf("internal error: agent path mismatch")
		}
	default:
		return fmt.Errorf("unknown target %q", target)
	}
	return nil
}
