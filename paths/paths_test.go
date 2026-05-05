package paths

import (
	"os"
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

func TestSeedInstructionFiles_copiesCoreInstructionFiles(t *testing.T) {
	root := t.TempDir()
	instruction := filepath.Join(root, "sessions", "s1")
	for _, name := range []string{"AGENT.md", "MEMORY.md", "SOUL.md", "USER.md"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := SeedInstructionFiles(root, instruction); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"AGENT.md", "MEMORY.md", "SOUL.md", "USER.md"} {
		b, err := os.ReadFile(filepath.Join(instruction, name))
		if err != nil {
			t.Fatalf("%s not copied: %v", name, err)
		}
		if got := string(b); got != name+"\n" {
			t.Fatalf("%s content = %q", name, got)
		}
	}
}

func TestSubSessionRoot(t *testing.T) {
	parent := filepath.Join("/u", "sessions", "ses")
	if got, want := SubSessionRoot(parent, "sub-abc"), filepath.Join(parent, "subs", "sub-abc"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestScheduledJobsPath(t *testing.T) {
	if got, want := ScheduledJobsPath("/home/u/.oneclaw"), filepath.Join("/home/u/.oneclaw", "scheduled_jobs.json"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
