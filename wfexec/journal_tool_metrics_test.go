package wfexec

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/workflow"
)

func TestResolveJournalPathForMetrics_nilRuntime(t *testing.T) {
	got, _, ok := resolveJournalPathForMetrics("", nil)
	if ok || got != "" {
		t.Fatalf("got %q ok=%v want empty,false", got, ok)
	}
}

func TestResolveJournalPathForMetrics_hostTurnNoParentFallback(t *testing.T) {
	tmp := t.TempDir()
	host := "default"
	corr := "c1"
	want := session.TurnRunJournalPath(tmp, host, corr)
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:    tmp,
			Turn:           engine.TurnContext{AgentID: host},
			CorrelationID:  corr,
			UserDataRoot:   filepath.Join(tmp, "ud"),
			InstructionRoot: filepath.Join(tmp, "instr"),
		},
	}
	got, _, ok := resolveJournalPathForMetrics("", rtx)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestResolveJournalPathForMetrics_requiresCorrelationForHostFallback(t *testing.T) {
	tmp := t.TempDir()
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:   tmp,
			Turn:          engine.TurnContext{AgentID: "default"},
			CorrelationID: "",
			UserDataRoot:  filepath.Join(tmp, "ud"),
		},
	}
	_, _, ok := resolveJournalPathForMetrics("", rtx)
	if ok {
		t.Fatal("expected not ok when correlation_id empty")
	}
}

func TestResolveJournalPathForMetrics_postTurnYAMLPriorityOverParentSession(t *testing.T) {
	tmpA := t.TempDir()
	tmpB := t.TempDir()
	host := "default"
	corr := "c-yaml"
	jpA := session.TurnRunJournalPath(tmpA, host, corr)
	jpB := session.TurnRunJournalPath(tmpB, host, corr)
	for _, p := range []string{jpA, jpB} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hostRTX := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:     tmpA,
			Turn:            engine.TurnContext{AgentID: host},
			CorrelationID:   corr,
			UserDataRoot:    filepath.Join(tmpA, "ud"),
			InstructionRoot: filepath.Join(tmpA, "instr"),
		},
	}
	yaml, err := BuildPostTurnCTXYAML(hostRTX)
	if err != nil {
		t.Fatal(err)
	}
	child := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:       filepath.Join(tmpA, "subs", "sub-x"),
			ParentSessionRoot: tmpB,
			ParentAgentType:   host,
			CorrelationID:     corr,
			UserDataRoot:      filepath.Join(tmpA, "ud"),
		},
	}
	got, _, ok := resolveJournalPathForMetrics(yaml, child)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != jpA {
		t.Fatalf("PostTurn path: got %q want %q (must not follow ParentSessionRoot to %q)", got, jpA, jpB)
	}
}

func TestHandleJournalToolMetrics_scansHostJournalViaParentSession(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	host := "default"
	corr := "c-metrics"
	udr := filepath.Join(tmp, "ud")
	jp := session.TurnRunJournalPath(tmp, host, corr)
	if err := os.MkdirAll(filepath.Dir(jp), 0o755); err != nil {
		t.Fatal(err)
	}
	line := `{"phase":"tool_call","detail":{"tool_call_id":"a","tool_name":"read_skill","arguments":"{}"}}` + "\n"
	if err := os.WriteFile(jp, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:       filepath.Join(tmp, "subs", "sub-run"),
			ParentSessionRoot: tmp,
			ParentAgentType:   host,
			CorrelationID:     corr,
			UserDataRoot:      udr,
		},
	}
	node := workflow.Node{
		Params: map[string]any{"match_tools": []any{"read_skill"}},
	}
	out, err := handleJournalToolMetrics(ctx, NodeInput{Text: ""}, NodeEnv{Runtime: rtx, Node: node})
	if err != nil {
		t.Fatal(err)
	}
	present, ok := out.Data["tool_matched_read_skill"].(bool)
	if !ok || !present {
		t.Fatalf("expected tool_matched_read_skill true, data=%v", out.Data)
	}
	if out.Data["distinct_tool_calls"] != 1 {
		t.Fatalf("distinct_tool_calls=%v want 1", out.Data["distinct_tool_calls"])
	}
	if jpGot, _ := out.Data["journal_path"].(string); jpGot != jp {
		t.Fatalf("journal_path=%q want %q", jpGot, jp)
	}
}

func TestResolveJournalPathForMetrics_prefersParentHostJournalUnderSubs(t *testing.T) {
	tmp := t.TempDir()
	host := "default"
	corr := "c-host"
	want := session.TurnRunJournalPath(tmp, host, corr)
	if err := os.MkdirAll(filepath.Dir(want), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(want, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	subSR := filepath.Join(tmp, "subs", "sub-skill-x")
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:       subSR,
			ParentSessionRoot: tmp,
			ParentAgentType:   host,
			CorrelationID:     corr,
			UserDataRoot:      filepath.Join(tmp, "ud"),
		},
	}
	got, _, ok := resolveJournalPathForMetrics("", rtx)
	if !ok {
		t.Fatal("expected ok")
	}
	if got != want {
		t.Fatalf("journal path: got %q want %q", got, want)
	}
}
