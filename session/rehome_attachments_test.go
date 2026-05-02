package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/mediastore"
	"github.com/lengzhao/oneclaw/tools"
)

func TestRehomeInboundAttachments_globalMediaToWorkspace(t *testing.T) {
	mediaRoot := t.TempDir()
	workspace := t.TempDir()
	scopeDir := filepath.Join(mediaRoot, "webchat-1", "chat", "msg1")
	if err := os.MkdirAll(scopeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	origName := "session-desktop-worker-provider-avd.zh.md"
	src := filepath.Join(scopeDir, origName)
	payload := []byte("# hello")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	e := NewEngine(workspace, tools.NewRegistry())
	e.MediaRoot = mediaRoot
	atts := []Attachment{
		{Name: origName, MIME: "text/markdown", Path: src},
	}
	if err := e.rehomeInboundAttachments(&atts); err != nil {
		t.Fatal(err)
	}
	if err := mediastore.ValidateRelPath(workspace, atts[0].Path); err != nil {
		t.Fatalf("expected path under workspace: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(workspace, filepath.FromSlash(atts[0].Path)))
	if err != nil || string(got) != string(payload) {
		t.Fatalf("read back: %v %q", err, got)
	}
}

func TestRehomeInboundAttachments_sameBasenameDistinctFiles(t *testing.T) {
	mediaRoot := t.TempDir()
	ws1 := t.TempDir()
	ws2 := t.TempDir()
	name := "dup.md"
	for i, ws := range []string{ws1, ws2} {
		scopeDir := filepath.Join(mediaRoot, "scope", string(rune('a'+i)))
		if err := os.MkdirAll(scopeDir, 0o755); err != nil {
			t.Fatal(err)
		}
		src := filepath.Join(scopeDir, name)
		if err := os.WriteFile(src, []byte("body"+string(rune('1'+i))), 0o644); err != nil {
			t.Fatal(err)
		}
		e := NewEngine(ws, tools.NewRegistry())
		e.MediaRoot = mediaRoot
		atts := []Attachment{{Name: name, Path: src}}
		if err := e.rehomeInboundAttachments(&atts); err != nil {
			t.Fatal(err)
		}
		if err := mediastore.ValidateRelPath(ws, atts[0].Path); err != nil {
			t.Fatal(err)
		}
	}
	// Both stored under inbound with unique random prefixes
	var rels [2]string
	for i, ws := range []string{ws1, ws2} {
		entries, err := os.ReadDir(filepath.Join(ws, "media", "inbound"))
		if err != nil {
			t.Fatal(err)
		}
		if len(entries) != 1 {
			t.Fatalf("ws%d: want 1 day bucket", i)
		}
		day := filepath.Join(ws, "media", "inbound", entries[0].Name())
		fs, err := os.ReadDir(day)
		if err != nil || len(fs) != 1 {
			t.Fatalf("ws%d files: %v %v", i, err, fs)
		}
		rels[i] = fs[0].Name()
	}
	if rels[0] == rels[1] {
		t.Fatalf("expected distinct stored filenames, got %q", rels[0])
	}
	if !strings.Contains(rels[0], "_") || !strings.Contains(rels[1], "_") {
		t.Fatalf("expected random_prefix_name form: %q %q", rels[0], rels[1])
	}
}

func TestRehomeInboundAttachments_skipsAlreadyUnderWorkspace(t *testing.T) {
	ws := t.TempDir()
	rel, err := mediastore.StoreBytes(ws, "x.md", []byte("z"), 1024)
	if err != nil {
		t.Fatal(err)
	}
	e := NewEngine(ws, tools.NewRegistry())
	atts := []Attachment{{Name: "x.md", Path: rel}}
	before := rel
	if err := e.rehomeInboundAttachments(&atts); err != nil {
		t.Fatal(err)
	}
	if atts[0].Path != before {
		t.Fatalf("should not re-copy: %q -> %q", before, atts[0].Path)
	}
}
