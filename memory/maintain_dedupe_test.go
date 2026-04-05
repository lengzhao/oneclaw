package memory

import (
	"strings"
	"testing"
)

func TestDedupeMaintenanceBullets(t *testing.T) {
	existing := "- user likes tabs\n- fix bug in foo"
	section := "## Auto-maintained (2026-04-05)\n- User likes tabs\n- New fact about bar\n- new fact about bar\n"
	out := dedupeMaintenanceBullets(section, existing)
	if !strings.Contains(out, "bar") || !strings.Contains(out, "Auto-maintained") {
		t.Fatalf("unexpected output: %s", out)
	}
	if strings.Contains(out, "likes tabs") {
		t.Fatalf("should drop duplicate: %s", out)
	}
}

func TestMaintenanceSectionOnlyNoDurable(t *testing.T) {
	s := "## Auto-maintained (2026-04-05)\n- (no durable entries)\n"
	if !maintenanceSectionOnlyNoDurable(s) {
		t.Fatal("expected only no durable")
	}
	s2 := "## Auto-maintained (2026-04-05)\n- something real\n"
	if maintenanceSectionOnlyNoDurable(s2) {
		t.Fatal("expected false")
	}
}
