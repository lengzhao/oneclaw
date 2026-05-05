package catalog

import (
	"strings"
	"testing"
)

func TestParseAgentMarkdown_frontmatter(t *testing.T) {
	raw := []byte(`---
agent_type: ignored_in_yaml
name: Foo Agent
tools:
  - echo
---
Hello **body**
`)
	a, err := ParseAgentMarkdown("foo", raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.AgentType != "foo" || a.Name != "Foo Agent" || len(a.Tools) != 1 {
		t.Fatalf("%+v", a)
	}
	if !strings.Contains(a.Body, "Hello") {
		t.Fatalf("body %q", a.Body)
	}
}

func TestParseAgentMarkdown_skillsRefs(t *testing.T) {
	raw := []byte(`---
skills:
  - skill-creator
---
body
`)
	a, err := ParseAgentMarkdown("x", raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.ReferencedSkillIDs) != 1 || a.ReferencedSkillIDs[0] != "skill-creator" {
		t.Fatalf("%v", a.ReferencedSkillIDs)
	}
}

func TestParseAgentMarkdown_workspaceAndMemoryInherit(t *testing.T) {
	raw := []byte(`---
name: Sub
workspace: private
inherit_parent_memory: true
---
body
`)
	a, err := ParseAgentMarkdown("sub1", raw)
	if err != nil {
		t.Fatal(err)
	}
	if a.Workspace != "private" || !a.InheritParentMemory {
		t.Fatalf("%+v", a)
	}
}

func TestParseAgentMarkdown_contextProfileDisable(t *testing.T) {
	raw := []byte(`---
name: Memory Extractor
context_profile:
  disable:
    - memory_recall
    - transcript
    - tasks
---
body
`)
	a, err := ParseAgentMarkdown("memory_extractor", raw)
	if err != nil {
		t.Fatal(err)
	}
	if !a.ContextProfile.Disabled("memory_recall") {
		t.Fatalf("expected memory_recall disabled: %+v", a.ContextProfile)
	}
	if !a.ContextProfile.Disabled("transcript") {
		t.Fatalf("expected transcript disabled: %+v", a.ContextProfile)
	}
	if a.ContextProfile.Disabled("skills") {
		t.Fatalf("skills should remain enabled by default: %+v", a.ContextProfile)
	}
}

func TestParseAgentMarkdown_noFrontmatterUsesStem(t *testing.T) {
	a, err := ParseAgentMarkdown("stemmy", []byte("plain"))
	if err != nil {
		t.Fatal(err)
	}
	if a.AgentType != "stemmy" {
		t.Fatal(a.AgentType)
	}
}
