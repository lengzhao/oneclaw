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

func TestInferHostJournalFromDelegation_ok(t *testing.T) {
	tmp := t.TempDir()
	host := "default"
	corr := "c-host"
	jp := filepath.Join(tmp, "runs", host, corr+".jsonl")
	if err := os.MkdirAll(filepath.Dir(jp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jp, []byte(`{}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subSR := filepath.Join(tmp, "subs", "sub-memory_extractor-test")
	child := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:       subSR,
			ParentSessionRoot: tmp,
			ParentAgentType:   host,
			CorrelationID:     corr,
			SessionSegment:    "wc-seg",
			UserDataRoot:      filepath.Join(tmp, "ud"),
			InstructionRoot:   filepath.Join(tmp, "instr"),
		},
	}
	r, err := inferHostJournalFromDelegation(child)
	if err != nil {
		t.Fatal(err)
	}
	if r.JournalPath != jp || r.SessionRoot != tmp || r.HostAgentID != host {
		t.Fatalf("unexpected %+v", r)
	}
	raw := "Context for this agent run:\n\nUser message:\nhi"
	got, err := resolveStructuredMemoryExtractInput(raw, child)
	if err != nil {
		t.Fatal(err)
	}
	if got.JournalPath != jp {
		t.Fatalf("resolve fallback journal: got %q want %q", got.JournalPath, jp)
	}
}

func TestPostTurnPromptHasJournalPath(t *testing.T) {
	tmp := t.TempDir()
	host := "default"
	corr := "c77"
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:     tmp,
			Turn:            engine.TurnContext{AgentID: host},
			CorrelationID:   corr,
			UserDataRoot:    filepath.Join(tmp, "ud"),
			InstructionRoot: filepath.Join(tmp, "instr"),
		},
	}
	yml, err := BuildPostTurnCTXYAML(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if !postTurnPromptHasJournalPath(yml) {
		t.Fatal("expected built YAML to carry journal path")
	}
	if postTurnPromptHasJournalPath("Context for this agent run:\n\nUser message:\nhi") {
		t.Fatal("plain fallback prompt must not look like PostTurn")
	}
	if postTurnPromptHasJournalPath("") {
		t.Fatal("empty")
	}
}
