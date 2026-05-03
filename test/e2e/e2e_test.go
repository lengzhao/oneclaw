package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/runner"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/setup"
	"github.com/lengzhao/oneclaw/subagent"
)

func stubReply() string {
	return "Hello from oneclaw stub model."
}

// default.turn from bootstrap ends with async memory_extractor / skill_generator goroutines; they race with t.TempDir()
// cleanup in short integration tests. For e2e we use the same steps minus async tails (see test/e2e_case.md E2E-09 note).
const e2eSyncDefaultTurn = `workflow_spec_version: 1
id: default.turn
description: E2E sync-only default.turn (no async child agents).
steps:
  - use: on_receive
  - use: load_prompt_md
  - use: load_memory_snapshot
  - use: list_skills
  - use: list_tasks
  - use: load_transcript
  - use: adk_main
  - use: on_respond
`

func bootstrapUserData(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := setup.Bootstrap(root); err != nil {
		t.Fatal(err)
	}
	wf := filepath.Join(root, "workflows", "default.turn.yaml")
	if err := os.WriteFile(wf, []byte(e2eSyncDefaultTurn), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func loadRunEnv(t *testing.T, root string, extraYAML string) *config.File {
	t.Helper()
	merged := []string{filepath.Join(root, "config.yaml")}
	if strings.TrimSpace(extraYAML) != "" {
		p := filepath.Join(t.TempDir(), "patch.yaml")
		if err := os.WriteFile(p, []byte(extraYAML), 0o644); err != nil {
			t.Fatal(err)
		}
		merged = append(merged, p)
	}
	cfg, err := config.LoadMerged(merged)
	if err != nil {
		t.Fatal(err)
	}
	config.ApplyEnvSecrets(cfg)
	config.PushRuntime(cfg)
	return cfg
}

func loadCatalog(t *testing.T, root string) *catalog.Catalog {
	t.Helper()
	cat, err := catalog.Load(filepath.Join(paths.CatalogRoot(root), "agents"))
	if err != nil {
		t.Fatal(err)
	}
	return cat
}

func loadManifest(t *testing.T, root string) *catalog.Manifest {
	t.Helper()
	mf, err := catalog.LoadManifest(paths.CatalogRoot(root))
	if err != nil {
		t.Fatal(err)
	}
	return mf
}

func pipeStdout(t *testing.T) (w *os.File, capture func() string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()
	return w, func() string {
		_ = w.Close()
		<-done
		_ = r.Close()
		return buf.String()
	}
}

func executeTurn(t *testing.T, root string, cfg *config.File, sess, prompt string, useMock bool, agentID string) (stdout string, err error) {
	t.Helper()
	w, capture := pipeStdout(t)
	defer func() { stdout = capture() }()
	p := runner.Params{
		Ctx:            context.Background(),
		UserDataRoot:   root,
		Config:         cfg,
		Catalog:        loadCatalog(t, root),
		Manifest:       loadManifest(t, root),
		AgentID:        agentID,
		SessionSegment: sess,
		UserPrompt:     prompt,
		UseMock:        useMock,
		Stdout:         w,
		CorrelationID:  subagent.NewCorrelationID(),
	}
	err = runner.ExecuteTurn(p)
	return stdout, err
}

func readRunEvents(t *testing.T, sessionRoot, agentType string) []session.RunEvent {
	t.Helper()
	path := filepath.Join(sessionRoot, "runs", agentType, "runs.jsonl")
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatal(err)
	}
	var out []session.RunEvent
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var e session.RunEvent
		if err := json.Unmarshal(line, &e); err != nil {
			t.Fatalf("runs.jsonl: %v", err)
		}
		out = append(out, e)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

func lastRunStartDetail(t *testing.T, sessionRoot, agentType string) map[string]any {
	t.Helper()
	evs := readRunEvents(t, sessionRoot, agentType)
	for i := len(evs) - 1; i >= 0; i-- {
		if evs[i].Phase == "run_start" && evs[i].Detail != nil {
			return evs[i].Detail
		}
	}
	t.Fatal("no run_start event")
	return nil
}

func transcriptLines(t *testing.T, sessionRoot string) int {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(sessionRoot, "transcript.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return 0
		}
		t.Fatal(err)
	}
	n := 0
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		if len(bytes.TrimSpace(sc.Bytes())) > 0 {
			n++
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestE2E_MockTurn_stdoutAndRunJournal(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, "")

	out, err := executeTurn(t, root, cfg, "e2e-basic", "ping", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, stubReply()) {
		t.Fatalf("stdout missing stub reply: %q", out)
	}

	sessRoot := paths.SessionRoot(root, "e2e-basic")
	d := lastRunStartDetail(t, sessRoot, "default")
	if v, ok := d["mock_llm"].(bool); !ok || !v {
		t.Fatalf("run_start mock_llm: %#v", d["mock_llm"])
	}
}

func TestE2E_MockTurn_emptyPromptFails(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, "")

	_, err := executeTurn(t, root, cfg, "e2e-empty", "", true, "")
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
	if !strings.Contains(err.Error(), "empty user prompt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2E_MockTurn_resetClearsTranscript(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, "")
	sess := "e2e-reset"

	_, err := executeTurn(t, root, cfg, sess, "first message", true, "")
	if err != nil {
		t.Fatal(err)
	}
	sessRoot := paths.SessionRoot(root, sess)
	if n := transcriptLines(t, sessRoot); n < 2 {
		t.Fatalf("want at least user+assistant lines after first turn, got %d", n)
	}

	out, err := executeTurn(t, root, cfg, sess, "/reset", true, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "已清除本会话的用户侧对话记录") {
		t.Fatalf("reset ack missing: %q", out)
	}
	if transcriptLines(t, sessRoot) != 0 {
		t.Fatal("transcript should be cleared after /reset")
	}

	_, err = executeTurn(t, root, cfg, sess, "after reset", true, "")
	if err != nil {
		t.Fatal(err)
	}
}

func TestE2E_MockTurn_sessionIsolation(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, "")

	if _, err := executeTurn(t, root, cfg, "sess-A", "hello A", true, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := executeTurn(t, root, cfg, "sess-B", "hello B", true, ""); err != nil {
		t.Fatal(err)
	}

	aPath := filepath.Join(paths.SessionRoot(root, "sess-A"), "transcript.jsonl")
	bPath := filepath.Join(paths.SessionRoot(root, "sess-B"), "transcript.jsonl")
	ab, _ := os.ReadFile(aPath)
	bb, _ := os.ReadFile(bPath)
	if strings.Contains(string(ab), "hello B") || strings.Contains(string(bb), "hello A") {
		t.Fatal("session transcripts leaked across sessions")
	}
}

func TestE2E_MockTurn_configProviderMockWithoutFlag(t *testing.T) {
	root := bootstrapUserData(t)
	patch := `
models:
  - id: default
    priority: 0
    provider: mock
    default_model: stub
`
	cfg := loadRunEnv(t, root, patch)

	out, err := executeTurn(t, root, cfg, "e2e-noflag", "hi", false, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, stubReply()) {
		t.Fatalf("stdout: %q", out)
	}
	d := lastRunStartDetail(t, paths.SessionRoot(root, "e2e-noflag"), "default")
	if v, ok := d["mock_llm"].(bool); !ok || !v {
		t.Fatalf("mock_llm detail: %#v", d["mock_llm"])
	}
}

func TestE2E_MockTurn_memoryRecallLogged(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, "")
	sess := "e2e-mem"

	var logBuf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(prev)
	t.Setenv("ONECLAW_VERBOSE_PROMPT", "1")

	sessRoot := paths.SessionRoot(root, sess)
	mm := memory.MonthUTC(time.Now().UTC())
	memPath := filepath.Join(sessRoot, "memory", mm, "e2e-recall.md")
	if err := os.MkdirAll(filepath.Dir(memPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memPath, []byte("# recall marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeTurn(t, root, cfg, sess, "ping", true, ""); err != nil {
		t.Fatal(err)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, "e2e-recall.md") && !strings.Contains(logs, "## Memory recall") {
		t.Fatalf("expected memory recall in verbose logs; logs snippet:\n%s", truncate(logs, 4000))
	}
}

func TestE2E_MockTurn_skillsReferencedInPromptLog(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, "")

	const skillID = "e2e-skill"
	skillDir := filepath.Join(root, "skills", skillID)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillMD := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillMD, []byte("---\nname: E2E Skill\n---\nSkill body.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	agentPath := filepath.Join(root, "agents", "skill_e2e.md")
	body := `---
name: E2E Skill Agent
skills:
  - ` + skillID + `
max_turns: 0
---
Test agent body.
`
	if err := os.WriteFile(agentPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	var logBuf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelInfo})))
	defer slog.SetDefault(prev)
	t.Setenv("ONECLAW_VERBOSE_PROMPT", "1")

	if _, err := executeTurn(t, root, cfg, "e2e-skill-sess", "ping", true, "skill_e2e"); err != nil {
		t.Fatal(err)
	}
	logs := logBuf.String()
	if !strings.Contains(logs, skillID) {
		t.Fatalf("expected skill id in verbose prompt logs; snippet:\n%s", truncate(logs, 4000))
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}
