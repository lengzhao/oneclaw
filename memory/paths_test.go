package memory

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMonthUTC(t *testing.T) {
	got := MonthUTC(time.Date(2026, 5, 3, 15, 0, 0, 0, time.FixedZone("CST", 8*3600)))
	if got != "2026-05" {
		t.Fatalf("MonthUTC: got %q", got)
	}
}

func TestRequireWriteUsesCurrentUTCMemoryMonth(t *testing.T) {
	fixed := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	if err := RequireWriteUsesCurrentUTCMemoryMonth("memory/2026-05/note.md", fixed); err != nil {
		t.Fatal(err)
	}
	if err := RequireWriteUsesCurrentUTCMemoryMonth("2026-05/other.md", fixed); err != nil {
		t.Fatal(err)
	}
	if err := RequireWriteUsesCurrentUTCMemoryMonth("memory/2025-01/x.md", fixed); err == nil {
		t.Fatal("expected error for wrong month")
	}
}

func TestNormalizeMemoryMonthRel(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"memory/2026-05/a.md", "memory/2026-05/a.md"},
		{"Memory/2026-05/a.md", "memory/2026-05/a.md"},
		{"./memory/2026-05/a.md", "memory/2026-05/a.md"},
		{"2026-05/a.md", "memory/2026-05/a.md"},
	}
	for _, tt := range tests {
		got, err := NormalizeMemoryMonthRel(tt.in)
		if err != nil || got != tt.want {
			t.Fatalf("NormalizeMemoryMonthRel(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	if _, err := NormalizeMemoryMonthRel("../x"); err == nil {
		t.Fatal("expected error for ..")
	}
	gotPH, err := NormalizeMemoryMonthRel("memory/YYYY-MM/placeholder.md")
	if err != nil {
		t.Fatal(err)
	}
	if want := "memory/" + MonthUTC(time.Now()) + "/placeholder.md"; gotPH != want {
		t.Fatalf("placeholder month: got %q want %q", gotPH, want)
	}
}

func TestResolveMemoryMonthMarkdown(t *testing.T) {
	root := filepath.Join(t.TempDir(), "instr")
	if _, err := ResolveMemoryMonthMarkdown("", "memory/2026-05/a.md"); err == nil {
		t.Fatal("expected error for empty root")
	}
	_, err := ResolveMemoryMonthMarkdown(root, "memory/2026-13/x.md")
	if err == nil {
		t.Fatal("expected error for invalid month")
	}
	_, err = ResolveMemoryMonthMarkdown(root, "other/2026-05/x.md")
	if err == nil {
		t.Fatal("expected error for wrong prefix")
	}
	got, err := ResolveMemoryMonthMarkdown(root, "memory/2026-05/note.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "memory", "2026-05", "note.md")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %s want %s", got, want)
	}
	got2, err := ResolveMemoryMonthMarkdown(root, "2026-05/other.md")
	if err != nil {
		t.Fatal(err)
	}
	want2 := filepath.Join(root, "memory", "2026-05", "other.md")
	if filepath.Clean(got2) != filepath.Clean(want2) {
		t.Fatalf("implicit memory prefix: got %s want %s", got2, want2)
	}
}

func TestResolveSkillsMarkdown(t *testing.T) {
	root := t.TempDir()
	_, err := ResolveSkillsMarkdown(root, "other/x.md")
	if err == nil {
		t.Fatal("expected error")
	}
	_, err = ResolveSkillsMarkdown(root, "skills/Demo/SKILL.md")
	if err == nil {
		t.Fatal("expected error for uppercase skill id")
	}
	got, err := ResolveSkillsMarkdown(root, "skills/demo/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, "skills", "demo", "SKILL.md")
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("got %s want %s", got, want)
	}
	got2, err := ResolveSkillsMarkdown(root, "skills/demo/scripts/run.sh")
	if err != nil {
		t.Fatal(err)
	}
	want2 := filepath.Join(root, "skills", "demo", "scripts", "run.sh")
	if filepath.Clean(got2) != filepath.Clean(want2) {
		t.Fatalf("got %s want %s", got2, want2)
	}
}
