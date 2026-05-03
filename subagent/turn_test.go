package subagent

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/tools"
)

func TestNewSubRunID_shape(t *testing.T) {
	reBare := regexp.MustCompile(`^sub-[0-9a-f]{16}$`)
	reTyped := regexp.MustCompile(`^sub-memory_extractor-[0-9a-f]{16}$`)
	reSan := regexp.MustCompile(`^sub-foo_bar-[0-9a-f]{16}$`)

	if id := newSubRunID(""); !reBare.MatchString(id) {
		t.Fatalf("empty agent: %q", id)
	}
	if id := newSubRunID("memory_extractor"); !reTyped.MatchString(id) {
		t.Fatalf("typed: %q", id)
	}
	if id := newSubRunID("foo/bar"); !reSan.MatchString(id) {
		t.Fatalf("sanitized: %q", id)
	}
	long := strings.Repeat("a", subRunAgentSegmentMaxRunes+20)
	id := newSubRunID(long)
	if want := "sub-" + strings.Repeat("a", subRunAgentSegmentMaxRunes) + "-"; !strings.HasPrefix(id, want) {
		t.Fatalf("truncate prefix mismatch: %q", id)
	}
	if !regexp.MustCompile(`^sub-a+-[0-9a-f]{16}$`).MatchString(id) {
		t.Fatalf("long agent: %q", id)
	}
}

func TestChildRegistryFromParent_echoOnly(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "ws")
	parent := tools.NewRegistry(ws)
	withEcho := append([]string{tools.ToolEcho}, tools.DefaultBuiltinIDs...)
	if err := tools.RegisterBuiltinsNamed(parent, withEcho); err != nil {
		t.Fatal(err)
	}
	child, err := ChildRegistryFromParent(parent, ws, []string{"echo"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := child.Names()
	if len(names) != 1 || names[0] != "echo" {
		t.Fatalf("got %v", names)
	}
}

func TestChildRegistryFromParent_emptyAllow_narrowTemplate(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "ws")
	parent := tools.NewRegistry(ws)
	if err := tools.RegisterBuiltins(parent); err != nil {
		t.Fatal(err)
	}
	child, err := ChildRegistryFromParent(parent, ws, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := slices.Clone(child.Names())
	slices.Sort(got)
	want := []string{"list_dir", "read_file"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestChildRegistryFromParent_emptyAllow_intersectsParent(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "ws")
	parent := tools.NewRegistry(ws)
	// Parent may register extra tools (e.g. echo); child empty allow → template ∩ parent only.
	if err := tools.RegisterBuiltinsNamed(parent, []string{
		tools.ToolEcho, tools.ToolReadFile, tools.ToolListDir,
	}); err != nil {
		t.Fatal(err)
	}
	child, err := ChildRegistryFromParent(parent, ws, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := slices.Clone(child.Names())
	slices.Sort(got)
	want := []string{tools.ToolListDir, tools.ToolReadFile}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestChildRegistryFromParent_unknownTool(t *testing.T) {
	ws := t.TempDir()
	parent := tools.NewRegistry(ws)
	if err := tools.RegisterBuiltins(parent); err != nil {
		t.Fatal(err)
	}
	if _, err := ChildRegistryFromParent(parent, ws, []string{"no_such_tool"}, nil); err == nil {
		t.Fatal("expected error")
	}
}
