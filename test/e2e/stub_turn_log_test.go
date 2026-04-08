//go:build e2e

package e2e_test

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

func turnLogPathToday(t *testing.T, cwd, home string) string {
	t.Helper()
	lay := memory.DefaultLayout(cwd, home)
	return memory.TurnLogPathForDate(lay, time.Now())
}

// E2E-98 无工具时仍写入 assistant_final 一行（按日分文件）
func TestE2E_98_TurnLogAssistantFinalWithoutTools(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "assistant for turn log"))
	e2eEnvWithMemory(t, stub)
	s := rtopts.Current()
	s.DisableMemoryExtract = false
	s.DisableTurnLog = false
	s.DisableMemoryAudit = true
	rtopts.Set(&s)
	e2eIsolateUserMemory(t, home)
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{
		Content:   "user for turn log",
		MessageID: "corr-98",
	}); err != nil {
		t.Fatal(err)
	}
	path := turnLogPathToday(t, cwd, home)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read turn log: %v", err)
	}
	line := strings.TrimSpace(string(raw))
	var rec map[string]any
	if err := json.Unmarshal([]byte(line), &rec); err != nil {
		t.Fatalf("json: %v\n%s", err, line)
	}
	if rec["kind"] != "assistant_final" {
		t.Fatalf("kind=%v", rec["kind"])
	}
	if rec["session_id"] != e.SessionID || rec["correlation_id"] != "corr-98" {
		t.Fatalf("%v", rec)
	}
	if !strings.Contains(rec["assistant_visible"].(string), "assistant for turn log") {
		t.Fatalf("assistant_visible=%v", rec["assistant_visible"])
	}
}

// E2E-100 工具一行 + 回合结束 assistant_final 一行
func TestE2E_100_TurnLogToolThenAssistantFinal(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionToolCalls("", []map[string]any{
		openaistub.ToolCall("call_1", "read_file", `{"path":"note.txt"}`),
	}))
	stub.Enqueue(openaistub.CompletionStop("", "done reading"))
	e2eEnvWithMemory(t, stub)
	s100 := rtopts.Current()
	s100.DisableMemoryExtract = false
	s100.DisableTurnLog = false
	s100.DisableMemoryAudit = true
	rtopts.Set(&s100)
	e2eIsolateUserMemory(t, home)
	if err := os.WriteFile(filepath.Join(cwd, "note.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := newStubEngine(t, stub, cwd)
	if err := e.SubmitUser(context.Background(), bus.InboundMessage{Content: "read note"}); err != nil {
		t.Fatal(err)
	}
	path := turnLogPathToday(t, cwd, home)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	sc := bufio.NewScanner(strings.NewReader(string(raw)))
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			lines = append(lines, t)
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Fatalf("want 2 lines (tool + assistant_final), got %d: %q", len(lines), string(raw))
	}
	var t0, t1 map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &t0)
	_ = json.Unmarshal([]byte(lines[1]), &t1)
	if t0["kind"] != "tool" || t0["name"] != "read_file" {
		t.Fatalf("line0=%v", t0)
	}
	if t1["kind"] != "assistant_final" || !strings.Contains(t1["assistant_visible"].(string), "done reading") {
		t.Fatalf("line1=%v", t1)
	}
	if t0["session_id"] != e.SessionID {
		t.Fatalf("session mismatch")
	}
}

// E2E-99 features.disable_turn_log 时不创建 turn-log 文件
func TestE2E_99_TurnLogDisabledNoFile(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	t.Setenv("HOME", home)
	stub := openaistub.New(t)
	stub.Enqueue(openaistub.CompletionStop("", "x"))
	e2eEnvWithMemory(t, stub)
	s99 := rtopts.Current()
	s99.DisableMemoryExtract = false
	s99.DisableTurnLog = true
	rtopts.Set(&s99)
	e2eIsolateUserMemory(t, home)
	eng := newStubEngine(t, stub, cwd)
	if err := eng.SubmitUser(context.Background(), bus.InboundMessage{Content: "y"}); err != nil {
		t.Fatal(err)
	}
	path := turnLogPathToday(t, cwd, home)
	if _, err := os.Stat(path); err == nil {
		t.Fatalf("turn log should not exist: %s", path)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
