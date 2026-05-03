package paths

import (
	"path/filepath"
	"testing"

	"github.com/lengzhao/oneclaw/config"
)

func TestResolveUserDataRoot_envAndConfig(t *testing.T) {
	t.Setenv(EnvUserDataRoot, "~/from-env")
	root, err := ResolveUserDataRoot(&config.File{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := filepath.Base(root), "from-env"; got != want {
		t.Fatalf("base=%q want %q", got, want)
	}
}

func TestInstructionRoot_isolate(t *testing.T) {
	const ud = "/u"
	if got := InstructionRoot(ud, "s1", true); got != SessionRoot(ud, "s1") {
		t.Fatalf("got %q", got)
	}
	if got := InstructionRoot(ud, "s1", false); got != ud {
		t.Fatalf("got %q", got)
	}
}

func TestWorkspace(t *testing.T) {
	if got, want := Workspace("/i"), filepath.Join("/i", "workspace"); got != want {
		t.Fatalf("got %q", got)
	}
}

func TestSubSessionRoot(t *testing.T) {
	parent := filepath.Join("/u", "sessions", "ses")
	if got, want := SubSessionRoot(parent, "sub-abc"), filepath.Join(parent, "subs", "sub-abc"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
