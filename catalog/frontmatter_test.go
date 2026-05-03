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

func TestParseAgentMarkdown_noFrontmatterUsesStem(t *testing.T) {
	a, err := ParseAgentMarkdown("stemmy", []byte("plain"))
	if err != nil {
		t.Fatal(err)
	}
	if a.AgentType != "stemmy" {
		t.Fatal(a.AgentType)
	}
}
