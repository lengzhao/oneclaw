package memory

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/loop"
)

func TestFormatMaintainToolDetail(t *testing.T) {
	got := formatMaintainToolDetail([]loop.ToolTraceEntry{
		{Step: 1, Name: "exec", OK: true, ArgsPreview: `{"cmd":"ls"}`},
		{Step: 2, Name: "exec", OK: false, Err: "exit 1"},
		{Step: 3, Name: "grep", OK: true},
	})
	if !strings.Contains(got, "step 1 exec ok") || !strings.Contains(got, "step 2 exec err") {
		t.Fatalf("missing steps: %q", got)
	}
	if !strings.Contains(got, "repeated_in_this_turn: exec×2") {
		t.Fatalf("expected repeat summary: %q", got)
	}
}

func TestFormatMaintainToolDetailEmpty(t *testing.T) {
	if formatMaintainToolDetail(nil) != "" {
		t.Fatal("expected empty")
	}
}
