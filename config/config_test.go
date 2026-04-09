package config

import (
	"os"
	"path/filepath"
	"testing"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"github.com/lengzhao/oneclaw/memory"
)

func boolPtr(b bool) *bool { return &b }

func TestMainAgentMaxSteps(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
agent:
  max_steps: 48
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(LoadOptions{Home: home, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if r.MainAgentMaxSteps() != 48 {
		t.Fatalf("got %d", r.MainAgentMaxSteps())
	}
	emptyCwd := t.TempDir()
	empty, err := Load(LoadOptions{Home: home, Cwd: emptyCwd})
	if err != nil {
		t.Fatal(err)
	}
	if empty.MainAgentMaxSteps() != 32 {
		t.Fatalf("default max steps: %d", empty.MainAgentMaxSteps())
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
agent:
  max_steps: 999
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r3, err := Load(LoadOptions{Home: home, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if r3.MainAgentMaxSteps() != 256 {
		t.Fatalf("clamp high: got %d", r3.MainAgentMaxSteps())
	}
}

func TestMerge_projectOverridesUser(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, memory.DotDir)
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`
model: from-user
openai:
  api_key: user-key
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
model: from-project
`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(LoadOptions{Home: home, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if r.ChatModel() != "from-project" {
		t.Fatalf("model: got %q want from-project", r.ChatModel())
	}
	if r.apiKeyResolved() != "user-key" {
		t.Fatalf("api key: got %q want user-key", r.apiKeyResolved())
	}
}

func TestMerge_explicitHighest(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`model: proj`), 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(t.TempDir(), "extra.yaml")
	if err := os.WriteFile(explicit, []byte(`model: extra-layer`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(LoadOptions{Home: home, Cwd: cwd, ExplicitPath: explicit})
	if err != nil {
		t.Fatal(err)
	}
	if r.ChatModel() != "extra-layer" {
		t.Fatalf("model: got %q", r.ChatModel())
	}
}

func TestLoad_explicitMissing(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	_, err := Load(LoadOptions{Home: home, Cwd: cwd, ExplicitPath: "/nonexistent/oneclaw-config.yaml"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMerge_mcpEnabled(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
mcp:
  enabled: true
  max_inline_text_runes: 1234
  servers:
    s1:
      enabled: true
      command: echo
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(LoadOptions{Home: home, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	if !r.MCPEnabled() {
		t.Fatal("MCPEnabled: want true")
	}
	m := r.MCP()
	if m.MaxInlineTextRunes != 1234 {
		t.Fatalf("MaxInlineTextRunes: got %d", m.MaxInlineTextRunes)
	}
	if len(m.Servers) != 1 || !m.Servers["s1"].Enabled || m.Servers["s1"].Command != "echo" {
		t.Fatalf("servers: %#v", m.Servers)
	}
}

func TestMerge_clawbridgeProjectOverridesUser(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	userDir := filepath.Join(home, memory.DotDir)
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "config.yaml"), []byte(`
clawbridge:
  clients:
    - id: user-bot
      driver: noop
      enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
clawbridge:
  clients:
    - id: proj-bot
      driver: noop
      enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(LoadOptions{Home: home, Cwd: cwd})
	if err != nil {
		t.Fatal(err)
	}
	cb, err := r.ClawbridgeConfigForRun()
	if err != nil {
		t.Fatal(err)
	}
	if len(cb.Clients) != 1 || cb.Clients[0].ID != "proj-bot" {
		t.Fatalf("clients: %#v", cb.Clients)
	}
}

func TestMerge_clawbridgeMediaRootFromExplicit(t *testing.T) {
	home := t.TempDir()
	cwd := t.TempDir()
	projDir := filepath.Join(cwd, memory.DotDir)
	if err := os.MkdirAll(projDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projDir, "config.yaml"), []byte(`
clawbridge:
  clients:
    - id: a
      driver: noop
      enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(t.TempDir(), "extra.yaml")
	if err := os.WriteFile(explicit, []byte(`
clawbridge:
  media:
    root: /tmp/explicit-media
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(LoadOptions{Home: home, Cwd: cwd, ExplicitPath: explicit})
	if err != nil {
		t.Fatal(err)
	}
	cb, err := r.ClawbridgeConfigForRun()
	if err != nil {
		t.Fatal(err)
	}
	if cb.Media.Root != "/tmp/explicit-media" {
		t.Fatalf("media root: got %q", cb.Media.Root)
	}
	if len(cb.Clients) != 1 {
		t.Fatalf("clients lost: %#v", cb.Clients)
	}
}

func TestClawbridgeConfigForRun_defaultMediaRoot(t *testing.T) {
	cwd := t.TempDir()
	r := &Resolved{
		merged: File{
			Clawbridge: cbconfig.Config{
				Clients: []cbconfig.ClientConfig{{ID: "x", Driver: "noop", Enabled: true}},
			},
		},
		cwd: cwd,
	}
	cb, err := r.ClawbridgeConfigForRun()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cwd, memory.DotDir, "media")
	if cb.Media.Root != want {
		t.Fatalf("default media root: got %q want %q", cb.Media.Root, want)
	}
}

func TestMergeMCP_laterDisables(t *testing.T) {
	var dst MCPFile
	mergeMCP(&dst, MCPFile{Enabled: boolPtr(true)})
	mergeMCP(&dst, MCPFile{Enabled: boolPtr(false)})
	var f File
	f.MCP = dst
	res := &Resolved{merged: f}
	if res.MCPEnabled() {
		t.Fatal("expected MCP off after false layer")
	}
}

func TestNotifyAuditSinkPaths_defaults(t *testing.T) {
	var r *Resolved
	llm, orch, vis := r.NotifyAuditSinkPaths()
	if !llm || !orch || !vis {
		t.Fatalf("nil resolved: %v %v %v", llm, orch, vis)
	}
	empty := &Resolved{merged: File{}}
	llm, orch, vis = empty.NotifyAuditSinkPaths()
	if !llm || !orch || !vis {
		t.Fatalf("empty file: %v %v %v", llm, orch, vis)
	}
}

func TestNotifyAuditSinkPaths_masterOff(t *testing.T) {
	f := File{}
	f.Features.DisableAuditSinks = boolPtr(true)
	r := &Resolved{merged: f}
	llm, orch, vis := r.NotifyAuditSinkPaths()
	if llm || orch || vis {
		t.Fatalf("want all off, got %v %v %v", llm, orch, vis)
	}
}

func TestNotifyAuditSinkPaths_perPath(t *testing.T) {
	f := File{}
	f.Features.DisableAuditLLM = boolPtr(true)
	r := &Resolved{merged: f}
	llm, orch, vis := r.NotifyAuditSinkPaths()
	if llm || !orch || !vis {
		t.Fatalf("got llm=%v orch=%v vis=%v", llm, orch, vis)
	}
}

func TestSessionTranscriptPaths(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "proj")
	f := File{}
	r := &Resolved{merged: f, cwd: cwd}
	tp, wp := r.SessionTranscriptPaths("abc123")
	wantT := filepath.Join(cwd, memory.DotDir, "sessions", "abc123", "transcript.json")
	wantW := filepath.Join(cwd, memory.DotDir, "sessions", "abc123", "working_transcript.json")
	if tp != wantT || wp != wantW {
		t.Fatalf("got transcript=%q working=%q", tp, wp)
	}
	f.Features.DisableTranscript = boolPtr(true)
	r2 := &Resolved{merged: f, cwd: cwd}
	tp2, wp2 := r2.SessionTranscriptPaths("abc123")
	if tp2 != "" || wp2 != "" {
		t.Fatalf("disabled transcript: got %q %q", tp2, wp2)
	}
}

func TestSessionWorkerCount(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "proj")
	r := &Resolved{merged: File{}, cwd: cwd}
	if r.SessionWorkerCount() != 0 {
		t.Fatalf("unset: %d", r.SessionWorkerCount())
	}
	f := File{}
	f.Sessions.WorkerCount = 16
	r2 := &Resolved{merged: f, cwd: cwd}
	if r2.SessionWorkerCount() != 16 {
		t.Fatalf("got %d", r2.SessionWorkerCount())
	}
}

func TestSessionsSQLitePath(t *testing.T) {
	cwd := filepath.Join(t.TempDir(), "proj")
	f := File{}
	r := &Resolved{merged: f, cwd: cwd}
	got := r.SessionsSQLitePath()
	want := filepath.Join(cwd, memory.DotDir, "sessions.sqlite")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	f.Sessions.DisableSQLite = boolPtr(true)
	r2 := &Resolved{merged: f, cwd: cwd}
	if r2.SessionsSQLitePath() != "" {
		t.Fatal("expected empty when disabled")
	}
	f = File{}
	f.Sessions.SQLitePath = "custom.db"
	r3 := &Resolved{merged: f, cwd: cwd}
	if r3.SessionsSQLitePath() != filepath.Join(cwd, "custom.db") {
		t.Fatalf("relative: %q", r3.SessionsSQLitePath())
	}
}

func TestMultimodalFeatureFlagsResolved(t *testing.T) {
	var r *Resolved
	if r.MultimodalImageDisabled() || r.MultimodalAudioDisabled() {
		t.Fatal("nil resolved should not disable multimodal")
	}
	f := File{}
	r2 := &Resolved{merged: f}
	if r2.MultimodalImageDisabled() || r2.MultimodalAudioDisabled() {
		t.Fatal("unset flags should default off (multimodal allowed)")
	}
	f.Features.DisableMultimodalImage = boolPtr(true)
	f.Features.DisableMultimodalAudio = boolPtr(true)
	r3 := &Resolved{merged: f}
	if !r3.MultimodalImageDisabled() || !r3.MultimodalAudioDisabled() {
		t.Fatalf("expected both disabled, img=%v audio=%v", r3.MultimodalImageDisabled(), r3.MultimodalAudioDisabled())
	}
}
