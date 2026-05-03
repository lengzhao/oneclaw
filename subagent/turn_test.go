package subagent

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/lengzhao/oneclaw/tools"
)

func TestChildRegistryFromParent_echoOnly(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "ws")
	parent := tools.NewRegistry(ws)
	if err := tools.RegisterBuiltins(parent); err != nil {
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
	want := []string{"echo", "list_dir", "read_file"}
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestChildRegistryFromParent_emptyAllow_intersectsParent(t *testing.T) {
	ws := filepath.Join(t.TempDir(), "ws")
	parent := tools.NewRegistry(ws)
	if err := tools.RegisterBuiltinsNamed(parent, []string{"echo"}); err != nil {
		t.Fatal(err)
	}
	child, err := ChildRegistryFromParent(parent, ws, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := child.Names()
	if len(names) != 1 || names[0] != "echo" {
		t.Fatalf("got %v", names)
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
