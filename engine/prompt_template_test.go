package engine

import (
	"testing"

	"github.com/lengzhao/oneclaw/session"
)

func TestRuntimeContext_PromptTemplate_roundTrip(t *testing.T) {
	var rtx RuntimeContext
	rtx.SetPromptTemplateEntry("k", "v")
	if rtx.PromptTemplateData == nil || rtx.PromptTemplateData["k"] != "v" {
		t.Fatalf("set failed %#v", rtx.PromptTemplateData)
	}
	raw, ok := rtx.PromptTemplateRaw("k")
	if !ok || raw != "v" {
		t.Fatalf("raw got %v ok=%v", raw, ok)
	}
	cp := rtx.PromptTemplateDataCopy()
	if len(cp) != 1 || cp["k"] != "v" {
		t.Fatalf("copy %#v", cp)
	}
	cp["k"] = "mutated"
	if rtx.PromptTemplateData["k"] != "v" {
		t.Fatal("copy should be shallow independent mutation check failed")
	}
}

func TestRuntimeContext_SetTranscriptReplayTurns_nilSafe(t *testing.T) {
	var rtx *RuntimeContext
	rtx.SetTranscriptReplayTurns(nil) // no panic
	var r2 RuntimeContext
	r2.SetTranscriptReplayTurns([]session.TranscriptTurn{{Role: "user", Content: "x"}})
	if len(r2.TranscriptReplayTurns) != 1 {
		t.Fatal(r2.TranscriptReplayTurns)
	}
}
