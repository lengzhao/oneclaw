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
	g := Global{
		MaxPromptBytes:  900_000,
		MinTailMessages: 6,
		RecallMaxBytes:  12_000,
	}
	if g.RecallBytes() != 12000 {
		t.Fatalf("RecallBytes=%d want 12000", g.RecallBytes())
	}
}

func TestGlobal_disabled(t *testing.T) {
	g := Global{MaxPromptBytes: 0, MinTailMessages: 4}
	if g.Enabled() {
		t.Fatal("expected disabled")
	}
}
