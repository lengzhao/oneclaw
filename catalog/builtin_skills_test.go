package catalog

import (
	"strings"
	"testing"
)

func TestReadBuiltinSkillSKILL_skillCreator(t *testing.T) {
	b, err := ReadBuiltinSkillSKILL("skill-creator")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.Contains(s, "SKILL.md") {
		head := s
		if len(head) > 200 {
			head = head[:200]
		}
		t.Fatalf("unexpected content: %s", head)
	}
}

func TestReadBuiltinSkillSKILL_invalid(t *testing.T) {
	if _, err := ReadBuiltinSkillSKILL("../x"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := ReadBuiltinSkillSKILL(""); err == nil {
		t.Fatal("expected error")
	}
}
