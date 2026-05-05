package e2e_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
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
const e2eSyncDefaultTurn = `workflow_spec_version: 2
id: default.turn
description: E2E sync-only default.turn (no async child agents).
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
end: respond
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
	config.ApplyUserDataSecrets(root, cfg)
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

func executeTurnCtx(t *testing.T, ctx context.Context, root string, cfg *config.File, sess, prompt string, useMock bool, agentID string) (stdout string, err error) {
	t.Helper()
	if ctx == nil {
		ctx = context.Background()
	}
	w, capture := pipeStdout(t)
	defer func() { stdout = capture() }()
	p := runner.Params{
		Ctx:            ctx,
		UserDataRoot:   root,
		Config:         cfg,
		Catalog: loadCatalog(t, root),
		AgentID: agentID,
		SessionSegment: sess,
		UserPrompt:     prompt,
		UseMock:        useMock,
		Stdout:         w,
		CorrelationID:  subagent.NewCorrelationID(),
	}
	err = runner.ExecuteTurn(p)
	return stdout, err
}

func executeTurn(t *testing.T, root string, cfg *config.File, sess, prompt string, useMock bool, agentID string) (stdout string, err error) {
	t.Helper()
	ctx := context.Background()
	var cancel context.CancelFunc
	if !useMock {
		ctx, cancel = context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
	}
	return executeTurnCtx(t, ctx, root, cfg, sess, prompt, useMock, agentID)
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
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	mock := useMockLLM(t)

	out, err := executeTurn(t, root, cfg, "e2e-basic", "ping", mock, "")
	if err != nil {
		t.Fatal(err)
	}
	sessRoot := paths.SessionRoot(root, "e2e-basic")
	d := lastRunStartDetail(t, sessRoot, "default")
	if mock {
		if !strings.Contains(out, stubReply()) {
			t.Fatalf("stdout missing stub reply: %q", out)
		}
		if v, ok := d["mock_llm"].(bool); !ok || !v {
			t.Fatalf("run_start mock_llm: %#v", d["mock_llm"])
		}
		assertRunJournalHasPhase(t, sessRoot, "default", "run_complete")
	} else {
		if len(strings.TrimSpace(out)) < 4 {
			t.Fatalf("expected non-empty assistant stdout: %q", out)
		}
		if strings.Contains(out, stubReply()) {
			t.Fatalf("stdout still looks like stub while live mode: %q", out)
		}
		if v, ok := d["mock_llm"].(bool); ok && v {
			t.Fatalf("run_start should not be mock in live mode: %#v", d)
		}
		assertRunJournalHasPhase(t, sessRoot, "default", "run_complete")
	}
}

func TestE2E_MockTurn_emptyPromptFails(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))

	_, err := executeTurn(t, root, cfg, "e2e-empty", "", useMockLLM(t), "")
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
	if !strings.Contains(err.Error(), "empty user prompt") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestE2E_MockTurn_resetClearsTranscript(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "e2e-reset"
	mock := useMockLLM(t)

	_, err := executeTurn(t, root, cfg, sess, "first message", mock, "")
	if err != nil {
		t.Fatal(err)
	}
	sessRoot := paths.SessionRoot(root, sess)
	if n := transcriptLines(t, sessRoot); n < 2 {
		t.Fatalf("want at least user+assistant lines after first turn, got %d", n)
	}

	out, err := executeTurn(t, root, cfg, sess, "/reset", mock, "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "已清除本会话的用户侧对话记录") {
		t.Fatalf("reset ack missing: %q", out)
	}
	if transcriptLines(t, sessRoot) != 0 {
		t.Fatal("transcript should be cleared after /reset")
	}

	_, err = executeTurn(t, root, cfg, sess, "after reset", mock, "")
	if err != nil {
		t.Fatal(err)
	}
	if n := transcriptLines(t, sessRoot); n < 2 {
		t.Fatalf("after post-reset turn want ≥2 transcript lines (user+assistant), got %d", n)
	}
}

func TestE2E_MockTurn_sessionIsolation(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	mock := useMockLLM(t)

	if _, err := executeTurn(t, root, cfg, "sess-A", "hello A", mock, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := executeTurn(t, root, cfg, "sess-B", "hello B", mock, ""); err != nil {
		t.Fatal(err)
	}
	if n := transcriptLines(t, paths.SessionRoot(root, "sess-A")); n < 2 {
		t.Fatalf("sess-A transcript want ≥2 lines, got %d", n)
	}
	if n := transcriptLines(t, paths.SessionRoot(root, "sess-B")); n < 2 {
		t.Fatalf("sess-B transcript want ≥2 lines, got %d", n)
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
	if liveLLMEnabled() && !testing.Short() {
		t.Skip("uses YAML provider: mock; skip when ONECLAW_E2E_LIVE_LLM=1")
	}
	root := bootstrapUserData(t)
	patch := `
default_model: mock/stub
models:
  - id: default
    priority: 0
    provider: mock
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
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "e2e-mem"
	mock := useMockLLM(t)

	logBuf, restore := captureVerboseJSONLogs(t)
	defer restore()

	sessRoot := paths.SessionRoot(root, sess)
	mm := memory.MonthUTC(time.Now().UTC())
	memPath := filepath.Join(sessRoot, "memory", mm, "e2e-recall.md")
	if err := os.MkdirAll(filepath.Dir(memPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memPath, []byte("# recall marker\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := executeTurn(t, root, cfg, sess, "ping", mock, ""); err != nil {
		t.Fatal(err)
	}
	recs := parseSlogJSONRecords(t, logBuf.Bytes())
	sysText, chatText, ok := lastADKMainPair(recs)
	if !ok {
		t.Fatal("missing adk_main log pair")
	}
	if !strings.Contains(chatText, "## Memory recall") || !strings.Contains(chatText, "e2e-recall.md") {
		t.Fatalf("chat_messages missing memory recall:\n%s", truncate(chatText, 4000))
	}
	// Recall 走单独 user 消息（wfexec.adkMessagesForMain），不应塞进 system 指令文本。
	if strings.Contains(sysText, "## Memory recall") {
		t.Fatalf("system_prompt must not embed MemoryRecall block; snippet:\n%s", truncate(sysText, 2000))
	}
	segs := splitChatMessageSections(chatText)
	var recallSeg string
	for _, seg := range segs {
		if strings.Contains(seg, "## Memory recall") {
			recallSeg = seg
			break
		}
	}
	if recallSeg == "" || messageRolePrefix(recallSeg) != "user" {
		t.Fatalf("Memory recall must be a user-role message segment; got recallSeg=%q segs=%d", truncate(recallSeg, 300), len(segs))
	}
	assertRunJournalHasPhase(t, sessRoot, "default", "run_complete")
}

func TestE2E_MockTurn_skillsReferencedInPromptLog(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))

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

	logBuf, restore := captureVerboseJSONLogs(t)
	defer restore()

	if _, err := executeTurn(t, root, cfg, "e2e-skill-sess", "ping", useMockLLM(t), "skill_e2e"); err != nil {
		t.Fatal(err)
	}
	recs := parseSlogJSONRecords(t, logBuf.Bytes())
	sysText, chatText, ok := lastADKMainPair(recs)
	if !ok {
		t.Fatal("missing adk_main log pair")
	}
	if !strings.Contains(sysText, skillID) {
		t.Fatalf("referenced skill id should appear in system_prompt (catalog injection / digest); snippet:\n%s", truncate(sysText, 6000))
	}
	if !strings.Contains(chatText, "ping") {
		t.Fatalf("chat_messages should contain current user text only path test; got:\n%s", truncate(chatText, 2000))
	}
	assertRunJournalHasPhase(t, paths.SessionRoot(root, "e2e-skill-sess"), "skill_e2e", "run_complete")
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func TestE2E_MockTurn_secondTurn_transcriptReplayInChatLog(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "e2e-2stub"
	mock := useMockLLM(t)

	if _, err := executeTurn(t, root, cfg, sess, "first-stub-msg-xx", mock, ""); err != nil {
		t.Fatal(err)
	}

	logBuf, restore := captureVerboseJSONLogs(t)
	defer restore()

	if _, err := executeTurn(t, root, cfg, sess, "second-stub-msg-yy", mock, ""); err != nil {
		t.Fatal(err)
	}

	recs := parseSlogJSONRecords(t, logBuf.Bytes())
	_, chatText, ok := lastADKMainPair(recs)
	if !ok {
		t.Fatal("missing adk_main log pair on second turn")
	}
	if !strings.Contains(chatText, "first-stub-msg-xx") {
		t.Fatalf("missing prior user turn in chat_messages:\n%s", truncate(chatText, 6000))
	}
	if mock {
		if !strings.Contains(chatText, stubReply()) {
			t.Fatalf("missing prior assistant (stub) in chat_messages:\n%s", truncate(chatText, 6000))
		}
	} else {
		segs := splitChatMessageSections(chatText)
		if len(segs) < 3 || messageRolePrefix(segs[1]) != "assistant" {
			t.Fatalf("want transcript assistant in chat_messages:\n%s", truncate(chatText, 6000))
		}
		rest := strings.TrimSpace(segs[1])
		const ap = "assistant: "
		if !strings.HasPrefix(rest, ap) {
			t.Fatalf("seg[1] format: %s", truncate(rest, 200))
		}
		body := strings.TrimSpace(strings.TrimPrefix(rest, ap))
		if len(body) < 8 {
			t.Fatalf("expected non-trivial assistant replay, got %q", truncate(body, 400))
		}
	}
	if !strings.Contains(chatText, "second-stub-msg-yy") {
		t.Fatalf("missing current user turn:\n%s", truncate(chatText, 6000))
	}
	assertRunJournalHasPhase(t, paths.SessionRoot(root, sess), "default", "run_complete")
}

func TestE2E_MockTurn_runJournal_hasRunComplete(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "e2e-runcomplete"

	if _, err := executeTurn(t, root, cfg, sess, "ping run journal", useMockLLM(t), ""); err != nil {
		t.Fatal(err)
	}

	assertRunJournalHasPhase(t, paths.SessionRoot(root, sess), "default", "run_complete")
}

func phaseList(evs []session.RunEvent) []string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Phase)
	}
	return out
}

func assertRunJournalHasPhase(t *testing.T, sessionRoot, agentType, phase string) {
	t.Helper()
	evs := readRunEvents(t, sessionRoot, agentType)
	for _, e := range evs {
		if e.Phase == phase {
			return
		}
	}
	t.Fatalf("runs.jsonl missing phase %q; phases=%v", phase, phaseList(evs))
}
