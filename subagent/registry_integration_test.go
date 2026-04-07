package subagent_test

import (
	"testing"

	"github.com/lengzhao/oneclaw/subagent"
	"github.com/lengzhao/oneclaw/tools/builtin"
)

func TestFilterRegistry_allowlist(t *testing.T) {
	parent := builtin.DefaultRegistry()
	f, err := subagent.FilterRegistry(parent, []string{"read_file", "grep"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := f.Get("read_file"); !ok {
		t.Fatal("expected read_file")
	}
	if _, ok := f.Get("bash"); ok {
		t.Fatal("bash should be excluded")
	}
}

func TestWithoutMetaTools(t *testing.T) {
	parent := builtin.DefaultRegistry()
	out, err := subagent.WithoutMetaTools(parent, "run_agent", "fork_context")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := out.Get("run_agent"); ok {
		t.Fatal("run_agent stripped")
	}
	if _, ok := out.Get("read_file"); !ok {
		t.Fatal("read_file kept")
	}
}
