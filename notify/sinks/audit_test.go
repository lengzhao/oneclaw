package sinks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/notify"
	"github.com/openai/openai-go"
)

func TestAuditSinksWriteJSONL_omitDotDir(t *testing.T) {
	dir := t.TempDir()
	o := Options{CWD: dir, AgentID: "test-agent", OmitDotDir: true}
	llm := NewLLMAuditSink(o)
	ev := notify.NewEvent(notify.EventModelStepEnd, "")
	ev.SessionID = "s1"
	ev.TurnID = "t1"
	ev.Data = map[string]any{"step": 0}
	if err := llm.Emit(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, "audit", "test-agent", "llm")
	matches, err := filepath.Glob(filepath.Join(base, "*", "*", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("glob: %v", matches)
	}
}

func TestAuditSinksWriteJSONL(t *testing.T) {
	dir := t.TempDir()
	o := Options{CWD: dir, AgentID: "test-agent"}
	llm := NewLLMAuditSink(o)
	orch := NewOrchestrationAuditSink(o)
	vis := NewVisibleAuditSink(o, func() []openai.ChatCompletionMessageParamUnion {
		return []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("hi"),
			openai.AssistantMessage("hello"),
		}
	})

	ev := notify.NewEvent(notify.EventModelStepEnd, "")
	ev.SessionID = "s1"
	ev.TurnID = "t1"
	ev.Data = map[string]any{"step": 0, "ok": true}
	if err := llm.Emit(context.Background(), ev); err != nil {
		t.Fatal(err)
	}

	ev2 := notify.NewEvent(notify.EventInboundReceived, "")
	ev2.SessionID = "s1"
	ev2.TurnID = "t1"
	ev2.Data = map[string]any{"user_content": "hi"}
	if err := orch.Emit(context.Background(), ev2); err != nil {
		t.Fatal(err)
	}

	ev2b := notify.NewEvent(notify.EventMemoryTurnContext, "")
	ev2b.SessionID = "s1"
	ev2b.TurnID = "t1"
	ev2b.Data = map[string]any{"memory_enabled": true, "recall_block": "x", "recall_block_bytes": 1}
	if err := orch.Emit(context.Background(), ev2b); err != nil {
		t.Fatal(err)
	}

	ev3 := notify.NewEvent(notify.EventTurnComplete, "")
	ev3.SessionID = "s1"
	ev3.TurnID = "t1"
	ev3.Data = map[string]any{
		"tool_count": 0,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
			{"role": "assistant", "content": "hello"},
		},
	}
	if err := vis.Emit(context.Background(), ev3); err != nil {
		t.Fatal(err)
	}

	base := filepath.Join(dir, ".oneclaw", "audit", "test-agent")
	for _, sub := range []string{"llm", "orchestration", "visible"} {
		matches, err := filepath.Glob(filepath.Join(base, sub, "*", "*", "*.jsonl"))
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 {
			t.Fatalf("%s: %v", sub, matches)
		}
		raw, err := os.ReadFile(matches[0])
		if err != nil {
			t.Fatal(err)
		}
		if sub == "orchestration" {
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
				if line == "" {
					continue
				}
				var m map[string]any
				if err := json.Unmarshal([]byte(line), &m); err != nil {
					t.Fatalf("%s: %s", sub, line)
				}
				if m["kind"] == nil {
					t.Fatalf("%s: %s", sub, line)
				}
			}
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatalf("%s: %s", sub, raw)
		}
		if m["kind"] == nil {
			t.Fatalf("%s", raw)
		}
		if sub == "visible" {
			msgs, _ := m["messages"].([]any)
			if len(msgs) != 2 {
				t.Fatalf("visible messages: want 2 got %d: %s", len(msgs), raw)
			}
		}
	}
}

func TestAuditSinksWriteJSONL_perSessionDir(t *testing.T) {
	dir := t.TempDir()
	o := Options{CWD: dir, AgentID: "AGENT", AuditSessionID: "deadbeefcafe"}
	llm := NewLLMAuditSink(o)
	ev := notify.NewEvent(notify.EventModelStepEnd, "")
	ev.SessionID = "deadbeefcafe"
	ev.TurnID = "t1"
	ev.Data = map[string]any{"step": 0}
	if err := llm.Emit(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	base := filepath.Join(dir, ".oneclaw", "sessions", "deadbeefcafe", "audit", "AGENT", "llm")
	matches, err := filepath.Glob(filepath.Join(base, "*", "*", "*.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("glob: %v", matches)
	}
}

func TestLLMSinkIgnoresOtherEvents(t *testing.T) {
	dir := t.TempDir()
	s := NewLLMAuditSink(Options{CWD: dir, AgentID: "a"})
	ev := notify.NewEvent(notify.EventTurnComplete, "")
	if err := s.Emit(context.Background(), ev); err != nil {
		t.Fatal(err)
	}
	// no file should be created
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".jsonl") {
			t.Fatalf("unexpected %s", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
