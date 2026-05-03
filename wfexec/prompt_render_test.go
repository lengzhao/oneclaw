package wfexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/catalog"
	"github.com/lengzhao/oneclaw/engine"
)

func TestRenderMainAgentPrompt_includesNowUTCFromRunStartedAt(t *testing.T) {
	dir := t.TempDir()
	ir := filepath.Join(dir, "instr")
	if err := os.MkdirAll(ir, 0o755); err != nil {
		t.Fatal(err)
	}
	ud := filepath.Join(dir, "userdata")
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	fixed := time.Date(2026, 5, 3, 12, 0, 0, 0, time.UTC)
	rtx := &engine.RuntimeContext{
		InstructionRoot: ir,
		UserDataRoot:    ud,
		RunStartedAt:    fixed,
		Agent:           &catalog.Agent{AgentType: "default"},
	}
	out, err := RenderMainAgentPrompt(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Current time (UTC)") {
		t.Fatalf("missing time header:\n%s", out)
	}
	if !strings.Contains(out, "2026-05-03T12:00:00Z") {
		t.Fatalf("want RFC3339 instant from RunStartedAt, got:\n%s", out)
	}
	if strings.LastIndex(out, "## Current time (UTC)") <= strings.LastIndex(out, "## Tasks (todo.json)") {
		t.Fatalf("want current time section after tasks section:\n%s", out)
	}
}
