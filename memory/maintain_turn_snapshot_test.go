package memory

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
)

func TestFormatMaintainToolDetail(t *testing.T) {
	got := formatMaintainToolDetail([]loop.ToolTraceEntry{
		{Step: 1, Name: "bash", OK: true, ArgsPreview: `{"cmd":"ls"}`},
		{Step: 2, Name: "bash", OK: false, Err: "exit 1"},
		{Step: 3, Name: "grep", OK: true},
	})
	if !strings.Contains(got, "step 1 bash ok") || !strings.Contains(got, "step 2 bash err") {
		t.Fatalf("missing steps: %q", got)
	}
	if !strings.Contains(got, "repeated_in_this_turn: bash×2") {
		t.Fatalf("expected repeat summary: %q", got)
	}
}

func TestFormatMaintainToolDetailEmpty(t *testing.T) {
	if formatMaintainToolDetail(nil) != "" {
		t.Fatal("expected empty")
	}
}
