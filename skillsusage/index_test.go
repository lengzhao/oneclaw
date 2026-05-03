package skillsusage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRecordRank(t *testing.T) {
	dir := t.TempDir()
	skillsRoot := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Record(skillsRoot, "alpha", "write_skill_file"); err != nil {
		t.Fatal(err)
	}
	if err := Record(skillsRoot, "beta", "append_skill_file"); err != nil {
		t.Fatal(err)
	}
	if err := Record(skillsRoot, "beta", "write_skill_file"); err != nil {
		t.Fatal(err)
	}
	counts, lastUsed, err := Aggregate(skillsRoot)
	if err != nil {
		t.Fatal(err)
	}
	got := RankSkillIDs(counts, lastUsed, []string{"alpha", "beta", "gamma"})
	if len(got) != 3 {
		t.Fatalf("rank len: %v", got)
	}
	if got[0] != "beta" || got[1] != "alpha" || got[2] != "gamma" {
		t.Fatalf("unexpected order: %v", got)
	}
}

func TestAggregateMissingLog(t *testing.T) {
	dir := t.TempDir()
	skillsRoot := filepath.Join(dir, "skills")
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	c, lu, err := Aggregate(skillsRoot)
	if err != nil {
		t.Fatal(err)
	}
	if len(c) != 0 || len(lu) != 0 {
		t.Fatalf("want empty maps, got c=%v lu=%v", c, lu)
	}
}
