package memory

import (
	"strings"
	"testing"
)

func TestTrimEpisodicAutoMaintainSection_underCap(t *testing.T) {
	h := "## Auto-maintained (2026-04-10)"
	s := h + "\n- a\n- b"
	if got := trimEpisodicAutoMaintainSection(s, h, 1024); got != s {
		t.Fatalf("expected unchanged, got %q", got)
	}
}

func TestTrimEpisodicAutoMaintainSection_dropsOldest(t *testing.T) {
	h := "## Auto-maintained (2026-04-10)"
	b1 := "- " + strings.Repeat("a", 200)
	b2 := "- " + strings.Repeat("b", 20)
	s := h + "\n" + b1 + "\n" + b2
	max := len(h) + 1 + len(b2) // exactly one short bullet + header; two bullets exceed this
	got := trimEpisodicAutoMaintainSection(s, h, max)
	if strings.Contains(got, "aaaa") {
		t.Fatalf("expected oldest bullet dropped, got %q", got)
	}
	if !strings.Contains(got, "bbbb") {
		t.Fatalf("expected newer bullet kept, got %q", got)
	}
}
