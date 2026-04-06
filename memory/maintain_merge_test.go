package memory

import (
	"strings"
	"testing"
)

func TestFindSameDayAutoMaintainedSpan(t *testing.T) {
	date := "2026-04-06"
	h := digestHeaderForDate(date)
	md := "# MEMORY\n\n## Other\n- x\n\n" + h + "\n- first\n\n" + h + "\n- second\n\n## Auto-maintained (2026-04-05)\n- old\n"
	start, end, ok := findSameDayAutoMaintainedSpan(md, date)
	if !ok {
		t.Fatal("expected ok")
	}
	block := md[start:end]
	if !strings.HasPrefix(block, h) {
		t.Fatalf("block prefix: %q", block)
	}
	if !strings.Contains(block, "first") || !strings.Contains(block, "second") {
		t.Fatalf("missing bullets: %q", block)
	}
	if strings.Contains(block, "2026-04-05") {
		t.Fatal("should not include prior day section")
	}
}

func TestFindSameDayAutoMaintainedSpan_missing(t *testing.T) {
	_, _, ok := findSameDayAutoMaintainedSpan("# x\n", "2026-04-06")
	if ok {
		t.Fatal("expected false")
	}
}

func TestMergeSameDayAutoMaintainedBlocks(t *testing.T) {
	date := "2026-04-06"
	h := digestHeaderForDate(date)
	old := h + "\n- Alpha fact\n- Beta note\n"
	newSec := h + "\n- Beta note\n- Gamma new\n"
	older := "## Auto-maintained (2026-04-05)\n- legacy\n"
	got := mergeSameDayAutoMaintainedBlocks(old, newSec, h, older)
	if !strings.Contains(got, "Alpha fact") {
		t.Fatalf("lost old unique: %q", got)
	}
	if !strings.Contains(got, "Gamma new") {
		t.Fatalf("missing new: %q", got)
	}
	if strings.Count(got, "Beta note") > 1 {
		t.Fatalf("duplicate beta: %q", got)
	}
}

func TestMergeSameDayAutoMaintainedBlocks_noExisting(t *testing.T) {
	h := digestHeaderForDate("2026-04-06")
	newSec := h + "\n- Only\n"
	got := mergeSameDayAutoMaintainedBlocks("", newSec, h, "")
	if got != strings.TrimSpace(newSec) {
		t.Fatalf("got %q want trim %q", got, strings.TrimSpace(newSec))
	}
}
