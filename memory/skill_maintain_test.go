package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	lzservice "github.com/lengzhao/memory/service"

	"github.com/lengzhao/oneclaw/loop"
)

func TestPostTurnSkillMaintainTrigger(t *testing.T) {
	if PostTurnSkillMaintainTrigger(nil) {
		t.Fatal("nil turn")
	}
	if PostTurnSkillMaintainTrigger(&PostTurnInput{}) {
		t.Fatal("empty tools")
	}
	tools := make([]loop.ToolTraceEntry, 5)
	for i := range tools {
		tools[i] = loop.ToolTraceEntry{Step: i + 1, Name: "read_file", OK: true}
	}
	if !PostTurnSkillMaintainTrigger(&PostTurnInput{Tools: tools}) {
		t.Fatal("want true for 5 tools")
	}
	if !PostTurnSkillMaintainTrigger(&PostTurnInput{
		Tools: []loop.ToolTraceEntry{{Step: 1, Name: "invoke_skill", OK: true}},
	}) {
		t.Fatal("want true for invoke_skill")
	}
	if PostTurnSkillMaintainTrigger(&PostTurnInput{
		Tools: []loop.ToolTraceEntry{{Step: 1, Name: "read_file", OK: true}},
	}) {
		t.Fatal("want false for single tool")
	}
}

func TestSkillAugmentedSchemaPatch(t *testing.T) {
	p := skillAugmentedExtractionPrompt()
	if p == nil {
		t.Fatal("nil prompt")
	}
	for _, sub := range []string{`"skill"`, `"transient"`, `"knowledge"`} {
		if !strings.Contains(p.JSONSchema, sub) {
			t.Fatalf("schema missing %q: %s", sub, p.JSONSchema)
		}
	}
	for _, sub := range []string{"SKILL EXTRACTION", string(ExtractNamespaceSkill)} {
		if !strings.Contains(p.SystemPrompt, sub) {
			t.Fatalf("system prompt missing %q", sub)
		}
	}
}

func TestSkillExtractPostHookWritesAndStrips(t *testing.T) {
	dir := t.TempDir()
	lay := Layout{CWD: dir, Project: filepath.Join(dir, "memory")}
	hook := skillExtractPostHook(lay)
	in := []lzservice.ExtractedMemory{
		{Namespace: "knowledge", Title: "fact", Content: "hello", Importance: 80, Confidence: 0.9},
		{Namespace: ExtractNamespaceSkill, Title: "my-skill", Content: "---\nname: my-skill\ndescription: d\n---\n\n# x\n", Importance: 70, Confidence: 0.9},
	}
	out, err := hook(t.Context(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || string(out[0].Namespace) != "knowledge" {
		t.Fatalf("want single knowledge memory, got %+v", out)
	}
	path := filepath.Join(lay.DotOrDataRoot(), "skills", "my-skill", skillMarkdownFile)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "---\nname: my-skill\ndescription: d\n---\n\n# x\n" {
		t.Fatalf("unexpected file: %q", raw)
	}
}
