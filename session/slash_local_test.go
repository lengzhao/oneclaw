package session

import (
	"strings"
	"testing"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/memory"
)

func TestTrySlashLocalTurn_StatusAndPaths(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	e.SessionID = "test-sid"
	e.UserDataRoot = "/tmp/udr"
	e.TranscriptPath = "/tmp/t.jsonl"
	e.RecallState = memory.RecallState{
		SurfacedPaths: map[string]struct{}{"/a": {}},
		SurfacedBytes: 42,
	}

	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/status", Channel: "cli"})
	if !ok {
		t.Fatal("expected local slash")
	}
	for _, want := range []string{"test-sid", "cli", "TranscriptPath:", "Recall: 1"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("status reply missing %q:\n%s", want, reply)
		}
	}

	reply, ok = e.trySlashLocalTurn(bus.InboundMessage{Content: "/paths"})
	if !ok {
		t.Fatal("expected local slash")
	}
	if !strings.Contains(reply, "MemoryBase:") || !strings.Contains(reply, "Project:") {
		t.Fatalf("paths reply: %s", reply)
	}
}

func TestTrySlashLocalTurn_SessionSubcommand(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	e.SessionID = "x"
	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/session full"})
	if !ok || !strings.Contains(reply, "会话 ID: x") {
		t.Fatalf("got ok=%v reply=%q", ok, reply)
	}
}

func TestTrySlashLocalTurn_RecallReset(t *testing.T) {
	e := NewEngine(t.TempDir(), nil)
	e.RecallState = memory.RecallState{
		SurfacedPaths: map[string]struct{}{"/p": {}},
		SurfacedBytes: 99,
	}
	reply, ok := e.trySlashLocalTurn(bus.InboundMessage{Content: "/recall reset"})
	if !ok {
		t.Fatal("expected local slash")
	}
	if !strings.Contains(reply, "已重置") {
		t.Fatalf("reply: %q", reply)
	}
	if len(e.RecallState.SurfacedPaths) != 0 || e.RecallState.SurfacedBytes != 0 {
		t.Fatalf("recall not cleared: %+v", e.RecallState)
	}
}
