package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/routing"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// E2E-101 维护管道：维护请求的 user 文本含多日 daily log 分段与 project topic 摘录（读请求体断言）。
func TestE2E_101_MaintainPromptMultiDayLogAndTopicExcerpts(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "main turn e2e101"))
	date := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	section := "## Auto-maintained (" + date + ")\n- E2E101_NEW_FACT\n"
	stub.Enqueue(openaistub.CompletionStop("", section))

	e2eEnvWithMemory(t, stub)
	t.Setenv("ONCLAW_DISABLE_AUTO_MAINTENANCE", "0")
	t.Setenv("ONCLAW_MAINTENANCE_MODEL", "gpt-4o")
	t.Setenv("ONCLAW_MAINTENANCE_MIN_LOG_BYTES", "30")
	t.Setenv("ONCLAW_MAINTENANCE_LOG_DAYS", "4")

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

	e := newStubEngine(t, cwd)
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "E2E101_TODAY_MARKER ping"}); err != nil {
		t.Fatal(err)
	}

	bodies := stub.ChatRequestBodies()
	if len(bodies) < 2 {
		t.Fatalf("want >=2 chat requests (main+maintain), got %d", len(bodies))
	}
	maintainUser, err := openaistub.ChatRequestUserTextConcat(bodies[1])
	if err != nil {
		t.Fatalf("parse maintain request: %v", err)
	}
	for _, sub := range []string{
		"### Daily log " + date,
		"### Daily log " + yesterday,
		"E2E101_YESTERDAY_MARKER",
		"E2E101_TODAY_MARKER",
		"e2e101_topic.md",
		"E2E101_TOPIC_MARKER",
	} {
		if !strings.Contains(maintainUser, sub) {
			n := min(800, len(maintainUser))
			t.Fatalf("maintain prompt missing %q\n---\n%s", sub, maintainUser[:n])
		}
	}

	memPath := filepath.Join(memDir, "MEMORY.md")
	raw, err := os.ReadFile(memPath)
	if err != nil {
		t.Fatalf("read MEMORY.md: %v", err)
	}
	if !strings.Contains(string(raw), "E2E101_NEW_FACT") {
		t.Fatalf("expected new fact in MEMORY.md:\n%s", string(raw))
	}
}

// E2E-102 维护强去重：MEMORY.md 已有同义 bullet 时，维护输出全被去重则不再追加 Auto-maintained 段。
func TestE2E_102_MaintainDedupeSkipsAppendWhenNoNewBullets(t *testing.T) {
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "main e2e102"))
	date := time.Now().Format("2006-01-02")
	dupLine := "- E2E102_DUP_LINE\n"
	section := "## Auto-maintained (" + date + ")\n" + dupLine
	stub.Enqueue(openaistub.CompletionStop("", section))

	e2eEnvWithMemory(t, stub)
	t.Setenv("ONCLAW_DISABLE_AUTO_MAINTENANCE", "0")
	t.Setenv("ONCLAW_MAINTENANCE_MODEL", "gpt-4o")
	t.Setenv("ONCLAW_MAINTENANCE_MIN_LOG_BYTES", "30")

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

	e := newStubEngine(t, cwd)
	if err := e.SubmitUser(context.Background(), routing.Inbound{Text: "E2E102_USER turn filler text for daily log"}); err != nil {
		t.Fatal(err)
	}

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
