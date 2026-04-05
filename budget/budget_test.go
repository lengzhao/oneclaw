package budget

import (
	"strings"
	"testing"
)

func TestTruncateUTF8_ascii(t *testing.T) {
	s := strings.Repeat("a", 100)
	out := TruncateUTF8(s, 50)
	if len(out) <= 50 {
		t.Fatalf("expected warning suffix over 50 bytes, got len=%d", len(out))
	}
	if !strings.Contains(out, "WARNING") {
		t.Fatalf("missing warning: %q", out)
	}
}

func TestRecallBytes_respectsCeil(t *testing.T) {
	t.Setenv("ONCLAW_DISABLE_CONTEXT_BUDGET", "")
	t.Setenv("ONCLAW_MAX_PROMPT_BYTES", "900000")
	t.Setenv("ONCLAW_RECALL_MAX_BYTES", "12000")
	g := FromEnv()
	if g.RecallBytes() != 12000 {
		t.Fatalf("RecallBytes=%d want 12000", g.RecallBytes())
	}
}

func TestFromEnv_disabled(t *testing.T) {
	t.Setenv("ONCLAW_DISABLE_CONTEXT_BUDGET", "1")
	g := FromEnv()
	if g.Enabled() {
		t.Fatal("expected disabled")
	}
}
