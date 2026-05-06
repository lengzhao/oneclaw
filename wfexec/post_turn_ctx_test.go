package wfexec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/session"
)

func TestBuildPostTurnCTXYAML_smoke(t *testing.T) {
	tmp := t.TempDir()
	host := "default"
	corr := "c1"
	jp := session.TurnRunJournalPath(tmp, host, corr)
	if err := os.MkdirAll(filepath.Dir(jp), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jp, []byte(`{"phase":"run_start"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rtx := &engine.RuntimeContext{
		TurnInputs: engine.TurnInputs{
			SessionRoot:     tmp,
			Turn:            engine.TurnContext{AgentID: host},
			CorrelationID:   corr,
			SessionSegment:  "seg",
			UserDataRoot:    filepath.Join(tmp, "ud"),
			InstructionRoot: filepath.Join(tmp, "instr"),
		},
	}
	y, err := BuildPostTurnCTXYAML(rtx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(y, "post_turn_ctx") || !strings.Contains(y, corr) || !strings.Contains(y, jp) {
		t.Fatalf("unexpected yaml:\n%s", y)
	}
}
