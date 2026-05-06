package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/setup"
)

// repoOneclawRoot resolves the oneclaw module directory (.../oneclaw) from this test file location.
func repoOneclawRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// file = .../oneclaw/test/e2e/skill_generator_template_integration_test.go
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func copyRepoWorkflowTemplate(t *testing.T, root string, relFromOneclaw string, destName string) {
	t.Helper()
	src := filepath.Join(repoOneclawRoot(t), filepath.FromSlash(relFromOneclaw))
	b, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read template %s: %v", src, err)
	}
	dest := filepath.Join(root, "workflows", destName)
	if err := os.WriteFile(dest, b, 0o644); err != nil {
		t.Fatal(err)
	}
}

// repoAsyncDefaultTurnYAML mirrors setup/templates/workflows/default.turn.yaml (async memory + skill agents).
const repoAsyncDefaultTurnYAML = `workflow_spec_version: 2
id: default.turn
description: Main turn v2 workflow.
nodes:
  receive:
    use: on_receive
    input: $start.user_prompt
  main:
    use: llm
    prompt: $start.user_prompt
  respond:
    use: on_respond
    input: $nodes.main
  memory_agent:
    use: agent_task
    async: true
    agent_type: memory_extractor
    input: $runtime.post_turn.ctx
    depends_on:
      - respond
  skill_agent:
    use: agent_task
    async: true
    agent_type: skill_generator
    input: $runtime.post_turn.ctx
    depends_on:
      - respond
end: respond
`

func bootstrapAsyncDefaultTurnAndRepoSkillGenerator(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := setup.Bootstrap(root); err != nil {
		t.Fatal(err)
	}
	wfDefault := filepath.Join(root, "workflows", "default.turn.yaml")
	if err := os.WriteFile(wfDefault, []byte(repoAsyncDefaultTurnYAML), 0o644); err != nil {
		t.Fatal(err)
	}
	copyRepoWorkflowTemplate(t, root, "setup/templates/workflows/skill_generator.yaml", "skill_generator.yaml")
	return root
}

func waitSubsJournalContains(t *testing.T, sessionRoot, agentType, needle string, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	pattern := filepath.Join(sessionRoot, "subs", "*", "runs", agentType, "*.jsonl")
	for time.Now().Before(deadline) {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range matches {
			b, err := os.ReadFile(m)
			if err != nil || len(b) == 0 {
				continue
			}
			if strings.Contains(string(b), needle) {
				return m
			}
		}
		time.Sleep(40 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s journal under %s to contain %q", agentType, sessionRoot, needle)
	return ""
}

func TestIntegration_asyncSkillGeneratorTemplate_skipsLLMWhenMainTurnHasNoTools(t *testing.T) {
	root := bootstrapAsyncDefaultTurnAndRepoSkillGenerator(t)
	cfg := loadRunEnv(t, root, "")
	sess := "integration-skill-gate-sess"
	corr := "e2e-skill-gate-no-tools"
	sessionRoot := paths.SessionRoot(root, paths.SanitizeSessionPathSegment(sess))
	hostAgent := cfg.ResolvedDefaultAgent()

	stdout, err := executeTurnCtxWithCorrelationID(t, context.Background(), root, cfg, sess, "say hi in two words", true, "", corr)
	if err != nil {
		t.Fatalf("ExecuteTurn: %v\nstdout=%s", err, stdout)
	}
	if !strings.Contains(strings.ToLower(stdout), "hi") && !strings.Contains(strings.ToLower(stdout), "hello") {
		t.Logf("stdout (main stub may vary): %s", strings.TrimSpace(stdout))
	}

	hostJournal := session.TurnRunJournalPath(sessionRoot, hostAgent, corr)
	hostBody, err := os.ReadFile(hostJournal)
	if err != nil {
		t.Fatalf("read host journal %s: %v", hostJournal, err)
	}
	if strings.Contains(string(hostBody), `"phase":"tool_call"`) {
		t.Fatalf("expected host turn without tool_call phases; journal snippet:\n%s", string(hostBody))
	}

	waitSubsJournalContains(t, sessionRoot, "memory_extractor", `"phase":"run_complete"`, 15*time.Second)
	skillPath := waitSubsJournalContains(t, sessionRoot, "skill_generator", `"phase":"run_complete"`, 15*time.Second)

	skillBody, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(skillBody), `"phase":"assistant_message"`) {
		t.Fatalf("skill_generator should not run llm when gate is false (no assistant_message); journal=%s\n%s",
			skillPath, string(skillBody))
	}
}
