package memory

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/budget"
)

func TestApplyTurnBudget_truncatedRecallAdjustsSurfacedBytes(t *testing.T) {
	rec := strings.Repeat("x", 200)
	st := &RecallState{
		SurfacedPaths: map[string]struct{}{"id1": {}},
		SurfacedBytes: len(rec),
	}
	b := TurnBundle{
		RecallBlock:   rec,
		UpdatedRecall: st,
		SystemSuffix:  "sys",
		AgentMdBlock:  "agent",
	}
	g := budget.Global{
		MaxPromptBytes: 500_000,
		RecallMaxBytes: 80,
	}
	ApplyTurnBudget(&b, g)
	if len(b.RecallBlock) >= len(rec) {
		t.Fatalf("expected recall truncation")
	}
	if b.UpdatedRecall.SurfacedBytes != len(b.RecallBlock) {
		t.Fatalf("SurfacedBytes=%d want %d (truncated attachment bytes)", b.UpdatedRecall.SurfacedBytes, len(b.RecallBlock))
	}
}
