package catalog

import (
	"testing"
)

func TestBuiltinSkillGeneratorReferencesSkillCreator(t *testing.T) {
	cat, err := Load("")
	if err != nil {
		t.Fatal(err)
	}
	ag := cat.Get("skill_generator")
	if ag == nil {
		t.Fatal("missing builtin skill_generator")
	}
	found := false
	for _, id := range ag.ReferencedSkillIDs {
		if id == "skill-creator" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("skill_generator must reference skill-creator, got %v", ag.ReferencedSkillIDs)
	}
}
