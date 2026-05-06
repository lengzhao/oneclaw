package wfexec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
)

func TestValidateRunJournalPath_ok(t *testing.T) {
	sr := t.TempDir()
	host := "default"
	jp := filepath.Join(sr, "runs", host, "x.jsonl")
	if err := validateRunJournalPath(jp, sr, host); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRunJournalPath_rejectsOtherAgentDir(t *testing.T) {
	sr := t.TempDir()
	host := "default"
	jp := filepath.Join(sr, "runs", "other", "x.jsonl")
	if err := validateRunJournalPath(jp, sr, host); err == nil {
		t.Fatal("expected error")
	}
}

func TestResolveStructuredMemoryExtract_roundtripYAML(t *testing.T) {
	tmp := t.TempDir()
	host := "default"
	corr := "c1"
	jp := filepath.Join(tmp, "runs", host, corr+".jsonl")
	if err := os.MkdirAll(filepath.Dir(jp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jp, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hostRTX := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:     tmp,
			Turn:            engine.TurnContext{AgentID: host},
			CorrelationID:   corr,
			SessionSegment:  "seg",
			UserDataRoot:    filepath.Join(tmp, "ud"),
			InstructionRoot: filepath.Join(tmp, "instr"),
		},
	}
	raw, err := BuildPostTurnCTXYAML(hostRTX)
	if err != nil {
		t.Fatal(err)
	}
	fallback := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionSegment:  "fallback-seg",
			InstructionRoot: "/fallback-instr",
		},
	}
	r, err := resolveStructuredMemoryExtractInput(raw, fallback)
	if err != nil {
		t.Fatal(err)
	}
	if r.JournalPath != jp || r.SessionRoot != tmp || r.HostAgentID != host {
		t.Fatalf("unexpected resolved: %+v", r)
	}
	if r.SessionSegment != "seg" {
		t.Fatalf("session_segment: got %q", r.SessionSegment)
	}
	if r.InstructionRoot != filepath.Join(tmp, "instr") {
		t.Fatalf("instruction_root: got %q", r.InstructionRoot)
	}
}
