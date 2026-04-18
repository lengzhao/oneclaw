package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
)

func TestParseFrontmatter(t *testing.T) {
	const doc = `---
description: Hello
when_to_use: When testing
---
Body **here**.
`
	fm, body := ParseFrontmatter(doc)
	if fm.Description != "Hello" || fm.WhenToUse != "When testing" {
		t.Fatalf("fm=%+v", fm)
	}
	if body != "Body **here**." {
		t.Fatalf("body=%q", body)
	}
}

func TestLoadAllProjectOverridesUser(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userSkills := filepath.Join(home, memory.DotDir, "skills")
	projSkills := filepath.Join(cwd, memory.DotDir, "skills")
	_ = os.MkdirAll(filepath.Join(userSkills, "demo"), 0o755)
	_ = os.MkdirAll(filepath.Join(projSkills, "demo"), 0o755)
	_ = os.WriteFile(filepath.Join(userSkills, "demo", skillFileName), []byte(`---
description: from user
---
`), 0o644)
	_ = os.WriteFile(filepath.Join(projSkills, "demo", skillFileName), []byte(`---
description: from project
---
`), 0o644)

	all := LoadAll(cwd, home, false, "")
	if len(all) != 1 {
		t.Fatalf("len=%d", len(all))
	}
	if all[0].Description != "from project" {
		t.Fatalf("want project override, got %q", all[0].Description)
	}
}

func TestOrderSkillsAndRecent(t *testing.T) {
	all := []Skill{
		{Name: "a"}, {Name: "b"}, {Name: "c"},
	}
	ordered := OrderSkills(all, []string{"c", "x", "a"})
	if len(ordered) != 3 {
		t.Fatal(len(ordered))
	}
	if ordered[0].Name != "c" || ordered[1].Name != "a" || ordered[2].Name != "b" {
		t.Fatalf("order: %v", namesOf(ordered))
	}
}

func TestFormatIndexBudget(t *testing.T) {
	all := []Skill{
		{Name: "x", Description: "long description that should be truncated when budget is tiny", WhenToUse: "w"},
		{Name: "y", Description: "second"},
	}
	// Very small budget: should get at least one line (full or short)
	s := strings.Join(FormatIndexLines(all, 50), "\n")
	if s == "" {
		t.Fatal("empty")
	}
}

func TestRecordUse(t *testing.T) {
	cwd := t.TempDir()
	if err := RecordUse(cwd, "one", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := RecordUse(cwd, "two", false, ""); err != nil {
		t.Fatal(err)
	}
	if err := RecordUse(cwd, "one", false, ""); err != nil {
		t.Fatal(err)
	}
	rec, err := LoadRecent(RecentFilePath(cwd, false, ""))
	if err != nil {
		t.Fatal(err)
	}
	if len(rec.Entries) != 2 || rec.Entries[0].Name != "one" {
		t.Fatalf("entries=%+v", rec.Entries)
	}
	if rec.Entries[0].UseCount != 2 {
		t.Fatalf("use_count=%d", rec.Entries[0].UseCount)
	}
}

func namesOf(sk []Skill) []string {
	var o []string
	for _, s := range sk {
		o = append(o, s.Name)
	}
	return o
}
