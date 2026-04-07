package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
)

// E2E-101 近场维护：user prompt 仅含 Current turn snapshot + MEMORY 摘录，不含多日 daily log / project topic（盘中虽有文件也不注入）。
func TestE2E_101_PostTurnMaintainPromptSessionOnly(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "main turn e2e101"))
	date := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	section := "## Auto-maintained (" + date + ")\n- E2E101_NEW_FACT\n"
	stub.Enqueue(openaistub.CompletionStop("", section))

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
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "E2E101_TODAY_MARKER ping"}); err != nil {
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

	epPath := memory.ProjectEpisodeDailyPath(cwd, date)
	raw, err := os.ReadFile(epPath)
	if err != nil {
		t.Fatalf("read episodic digest: %v", err)
	}
	if !strings.Contains(string(raw), "E2E101_NEW_FACT") {
		t.Fatalf("expected new fact in daily digest:\n%s", string(raw))
	}
}

// E2E-113 远场维护 RunScheduledMaintain：user 为工具型任务说明（绝对路径），不内嵌 daily log / topic 全文。
func TestE2E_113_ScheduledMaintainPromptToolOrientedPaths(t *testing.T) {
	stub := openaistub.New(t)
	date := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	section := "## Auto-maintained (" + date + ")\n- E2E103_NEW_FACT\n"
	stub.Enqueue(openaistub.CompletionStop("", section))

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

	memDir := filepath.Join(cwd, memory.DotDir, "memory")
	if err := os.MkdirAll(memDir, 0o755); err != nil {
		t.Fatal(err)
	}
	topicPath := filepath.Join(memDir, "e2e103_topic.md")
	if err := os.WriteFile(topicPath, []byte("# topic\nE2E103_TOPIC_MARKER body\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	client := openai.NewClient(stubOpenAIOptions(stub)...)
	memory.RunScheduledMaintain(context.Background(), lay, &client, "gpt-4o", 512,
		&memory.ScheduledMaintainOpts{ToolRegistry: builtin.ScheduledMaintainReadRegistry()})

	bodies := stub.ChatRequestBodies()
	if len(bodies) < 1 {
		t.Fatalf("want scheduled maintain chat request, got %d", len(bodies))
	}
	maintainUser, err := openaistub.ChatRequestUserTextConcat(bodies[0])
	if err != nil {
		t.Fatalf("parse maintain request: %v", err)
	}
	rulesMem := filepath.Join(memDir, "MEMORY.md")
	epPath := memory.ProjectEpisodeDailyPath(cwd, date)
	todayLog := filepath.Clean(memory.DailyLogPath(lay.Auto, date))
	for _, sub := range []string{
		"far-field",
		"read_file",
		"write_behavior_policy",
		filepath.Clean(lay.Auto),
		todayLog,
		filepath.Clean(rulesMem),
		filepath.Clean(epPath),
		filepath.Clean(lay.Project),
	} {
		if !strings.Contains(maintainUser, sub) {
			n := min(800, len(maintainUser))
			t.Fatalf("scheduled prompt missing %q\n---\n%s", sub, maintainUser[:n])
		}
	}
	for _, sub := range []string{
		"### Daily log " + date,
		"E2E103_YESTERDAY_MARKER",
		"E2E103_TODAY_MARKER",
		"E2E103_TOPIC_MARKER",
	} {
		if strings.Contains(maintainUser, sub) {
			n := min(800, len(maintainUser))
			t.Fatalf("scheduled user prompt must not embed log/topic body %q\n---\n%s", sub, maintainUser[:n])
		}
	}

	raw, err := os.ReadFile(epPath)
	if err != nil {
		t.Fatalf("read episodic digest: %v", err)
	}
	if !strings.Contains(string(raw), "E2E103_NEW_FACT") {
		t.Fatalf("expected new fact in daily digest:\n%s", string(raw))
	}
}

// E2E-102 维护强去重：规则 MEMORY.md 已有同义 bullet 时，维护输出全被去重则不再写入 episodic 段。
func TestE2E_102_MaintainDedupeSkipsAppendWhenNoNewBullets(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "main e2e102"))
	date := time.Now().Format("2006-01-02")
	dupLine := "- E2E102_DUP_LINE\n"
	section := "## Auto-maintained (" + date + ")\n" + dupLine
	stub.Enqueue(openaistub.CompletionStop("", section))

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
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "E2E102_USER turn filler text for daily log"}); err != nil {
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
