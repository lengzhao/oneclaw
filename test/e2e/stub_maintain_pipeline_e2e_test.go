//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-101 回合后 extract：user 侧为 lengzhao/memory 提取模板，仅含本回合快照 + 上下文；不注入多日 daily log / topic。
func TestE2E_101_PostTurnMaintainPromptSessionOnly(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "main turn e2e101"))
	date := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	extractJSON := `{"memories":[{"namespace":"knowledge","title":"e2e101","content":"E2E101_NEW_FACT","summary":"","tags":[],"importance":70,"confidence":0.92,"reasoning":"e2e"}]}`
	stub.Enqueue(openaistub.CompletionStop("", extractJSON))

	e2eEnvWithMemory(t, stub)
	s := rtopts.Current()
	s.DisableAutoMaintenance = false
	s.MaintenanceModel = "gpt-4o"
	s.PostTurnMinLogBytes = 30
	rtopts.Set(&s)

	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	e2eIsolateUserMemory(t, home)

	lay := memory.DefaultLayout(cwd, home)
	yPath := memory.DailyLogPath(lay.Auto, yesterday)
	if err := os.MkdirAll(filepath.Dir(yPath), 0o755); err != nil {
		t.Fatal(err)
	}
	yesterdayBody := strings.Repeat("y", 120) + " E2E101_YESTERDAY_MARKER\n"
	if err := os.WriteFile(yPath, []byte(yesterdayBody), 0o644); err != nil {
		t.Fatal(err)
	}

	memDir := filepath.Join(cwd, memory.DotDir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	topicPath := filepath.Join(memDir, "e2e101_topic.md")
	if err := os.WriteFile(topicPath, []byte("# topic\nE2E101_TOPIC_MARKER body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newStubEngine(t, stub, cwd)
	stubAttachPostTurnExtractLLM(e, stub)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "E2E101_TODAY_MARKER ping"}); err != nil {
		t.Fatal(err)
	}
	e2eWaitMinChatRequests(t, stub, 2, 5*time.Second)

	bodies := stub.ChatRequestBodies()
	if len(bodies) < 2 {
		t.Fatalf("want >=2 chat requests (main+maintain), got %d", len(bodies))
	}
	maintainUser, err := openaistub.ChatRequestUserTextConcat(bodies[1])
	if err != nil {
		t.Fatalf("parse maintain request: %v", err)
	}
	for _, sub := range []string{
		"## Dialog to analyze",
		"Current turn snapshot",
		"E2E101_TODAY_MARKER",
		"main turn e2e101",
	} {
		if !strings.Contains(maintainUser, sub) {
			n := min(800, len(maintainUser))
			t.Fatalf("maintain prompt missing %q\n---\n%s", sub, maintainUser[:n])
		}
	}
	for _, sub := range []string{
		"### Daily log " + date,
		"### Daily log " + yesterday,
		"E2E101_YESTERDAY_MARKER",
		"e2e101_topic.md",
		"E2E101_TOPIC_MARKER",
	} {
		if strings.Contains(maintainUser, sub) {
			n := min(800, len(maintainUser))
			t.Fatalf("near-field prompt must not contain %q\n---\n%s", sub, maintainUser[:n])
		}
	}

	sqlitePath := filepath.Join(lay.Auto, "agent_memory.sqlite")
	e2eWaitForFile(t, sqlitePath, 5*time.Second)
	e2eWaitAgentMemorySubstring(t, sqlitePath, "E2E101_NEW_FACT", 3*time.Second)
}

// E2E-113 远场维护 RunScheduledMaintain：lengzhao/memory Extract，user 含多日 log / topic 语料与规则摘要上下文。
func TestE2E_113_ScheduledMaintainPromptToolOrientedPaths(t *testing.T) {
	stub := openaistub.New(t)
	date := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	extractJSON := `{"memories":[{"namespace":"knowledge","title":"e2e103","content":"E2E103_NEW_FACT","summary":"","tags":[],"importance":70,"confidence":0.92,"reasoning":"e2e"}]}`
	stub.Enqueue(openaistub.CompletionStop("", extractJSON))

	baseStubTransport(t, stub)
	s113 := rtopts.Current()
	s113.MaintenanceModel = "gpt-4o"
	s113.MaintenanceMinLogBytes = 30
	s113.MaintenanceLogDays = 4
	rtopts.Set(&s113)

	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	e2eIsolateUserMemory(t, home)

	lay := memory.DefaultLayout(cwd, home)
	yPath := memory.DailyLogPath(lay.Auto, yesterday)
	if err := os.MkdirAll(filepath.Dir(yPath), 0o755); err != nil {
		t.Fatal(err)
	}
	yesterdayBody := strings.Repeat("y", 120) + " E2E103_YESTERDAY_MARKER\n"
	if err := os.WriteFile(yPath, []byte(yesterdayBody), 0o644); err != nil {
		t.Fatal(err)
	}
	tPath := memory.DailyLogPath(lay.Auto, date)
	todayBody := strings.Repeat("z", 150) + " E2E103_TODAY_MARKER\n"
	if err := os.WriteFile(tPath, []byte(todayBody), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}
	topicPath := filepath.Join(lay.Project, "e2e103_topic.md")
	if err := os.WriteFile(topicPath, []byte("# topic\nE2E103_TOPIC_MARKER body\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rulesMem := filepath.Join(lay.Project, "MEMORY.md")
	if err := os.WriteFile(rulesMem, []byte("# MEMORY\nE2E103_RULES_MARKER\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	base := strings.TrimSuffix(stub.BaseURL(), "/")
	extractLLM := memory.NewScheduledExtractLLM("sk-test-stub", base, "gpt-4o")
	memory.RunScheduledMaintain(context.Background(), lay, nil, "gpt-4o", 512, nil, extractLLM)

	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatalf("want scheduled maintain chat request, got %d", len(bodies))
	}
	uMsg, err := openaistub.ChatRequestUserTextConcat(bodies[0])
	if err != nil {
		t.Fatalf("parse maintain request: %v", err)
	}
	for _, sub := range []string{
		"Project MEMORY.md rules excerpt",
		"E2E103_RULES_MARKER",
		"Scheduled / far-field",
		"## Corpus",
		"### Daily log " + date,
		"### Daily log " + yesterday,
		"E2E103_YESTERDAY_MARKER",
		"E2E103_TODAY_MARKER",
		"### Topic e2e103_topic.md",
		"E2E103_TOPIC_MARKER",
	} {
		if !strings.Contains(uMsg, sub) {
			n := min(800, len(uMsg))
			t.Fatalf("scheduled extract user prompt missing %q\n---\n%s", sub, uMsg[:n])
		}
	}

	sqlitePath := filepath.Join(lay.Auto, "agent_memory.sqlite")
	e2eWaitForFile(t, sqlitePath, 5*time.Second)
	e2eWaitAgentMemorySubstring(t, sqlitePath, "E2E103_NEW_FACT", 3*time.Second)
}

// E2E-102 回合后 extract 返回空 memories 时不改规则 MEMORY.md。
func TestE2E_102_MaintainDedupeSkipsAppendWhenNoNewBullets(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "main e2e102"))
	stub.Enqueue(openaistub.CompletionStop("", `{"memories":[]}`))

	e2eEnvWithMemory(t, stub)
	s102 := rtopts.Current()
	s102.DisableAutoMaintenance = false
	s102.MaintenanceModel = "gpt-4o"
	s102.MaintenanceMinLogBytes = 30
	s102.PostTurnMinLogBytes = 30
	rtopts.Set(&s102)

	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	e2eIsolateUserMemory(t, home)

	memDir := filepath.Join(cwd, memory.DotDir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	memPath := filepath.Join(memDir, "MEMORY.md")
	seed := "# MEMORY\n\n- E2E102_DUP_LINE\n"
	if err := os.WriteFile(memPath, []byte(seed), 0o644); err != nil {
		t.Fatal(err)
	}

	e := newStubEngine(t, stub, cwd)
	stubAttachPostTurnExtractLLM(e, stub)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "E2E102_USER turn filler text for daily log"}); err != nil {
		t.Fatal(err)
	}
	e2eWaitMinChatRequests(t, stub, 2, 5*time.Second)

	after, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != seed {
		t.Fatalf("MEMORY.md should be unchanged after dedupe skip; before=%q after=%q", seed, string(after))
	}
	if strings.Contains(string(after), "## Auto-maintained") {
		t.Fatalf("did not expect Auto-maintained section: %s", string(after))
	}
}
