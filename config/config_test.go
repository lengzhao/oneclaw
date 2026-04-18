package config

import (
	"os"
	"path/filepath"
	"testing"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
)

func boolPtr(b bool) *bool { return &b }

func userConfigDir(home string) string {
	return filepath.Join(home, memory.DotDir)
}

func TestMainAgentMaxSteps(t *testing.T) {
	home := t.TempDir()
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
agent:
  max_steps: 48
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(LoadOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if r.MainAgentMaxSteps() != 48 {
		t.Fatalf("got %d", r.MainAgentMaxSteps())
	}
	emptyHome := t.TempDir()
	empty, err := Load(LoadOptions{Home: emptyHome})
	if err != nil {
		t.Fatal(err)
	}
	if empty.MainAgentMaxSteps() != 100 {
		t.Fatalf("default max steps: %d", empty.MainAgentMaxSteps())
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
agent:
  max_steps: 999
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r3, err := Load(LoadOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if r3.MainAgentMaxSteps() != 256 {
		t.Fatalf("clamp high: got %d", r3.MainAgentMaxSteps())
	}
}

func TestMainAgentMaxCompletionTokens(t *testing.T) {
	home := t.TempDir()
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
agent:
  max_tokens: 16384
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(LoadOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if r.MainAgentMaxCompletionTokens() != 16384 {
		t.Fatalf("got %d", r.MainAgentMaxCompletionTokens())
	}
	emptyHome := t.TempDir()
	empty, err := Load(LoadOptions{Home: emptyHome})
	if err != nil {
		t.Fatal(err)
	}
	if empty.MainAgentMaxCompletionTokens() != 32768 {
		t.Fatalf("default max tokens: %d", empty.MainAgentMaxCompletionTokens())
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
agent:
  max_tokens: 9999999
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r2, err := Load(LoadOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if r2.MainAgentMaxCompletionTokens() != 131072 {
		t.Fatalf("clamp high: got %d", r2.MainAgentMaxCompletionTokens())
	}
}

func TestMerge_explicitOverridesUser(t *testing.T) {
	home := t.TempDir()
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
model: from-user
openai:
  api_key: user-key
`), 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(t.TempDir(), "overlay.yaml")
	if err := os.WriteFile(explicit, []byte(`
model: from-project
`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(LoadOptions{Home: home, ExplicitPath: explicit})
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
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`model: proj`), 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(t.TempDir(), "extra.yaml")
	if err := os.WriteFile(explicit, []byte(`model: extra-layer`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(LoadOptions{Home: home, ExplicitPath: explicit})
	if err != nil {
		t.Fatal(err)
	}
	if r.ChatModel() != "extra-layer" {
		t.Fatalf("model: got %q", r.ChatModel())
	}
}

func TestLoad_explicitMissing(t *testing.T) {
	home := t.TempDir()
	_, err := Load(LoadOptions{Home: home, ExplicitPath: "/nonexistent/oneclaw-config.yaml"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestMerge_mcpEnabled(t *testing.T) {
	home := t.TempDir()
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
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
	r, err := Load(LoadOptions{Home: home})
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

func TestMerge_clawbridgeExplicitOverridesUser(t *testing.T) {
	home := t.TempDir()
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
clawbridge:
  clients:
    - id: user-bot
      driver: noop
      enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}
	explicit := filepath.Join(t.TempDir(), "cb.yaml")
	if err := os.WriteFile(explicit, []byte(`
clawbridge:
  clients:
    - id: proj-bot
      driver: noop
      enabled: true
`), 0o644); err != nil {
		t.Fatal(err)
	}

	r, err := Load(LoadOptions{Home: home, ExplicitPath: explicit})
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
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
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
	r, err := Load(LoadOptions{Home: home, ExplicitPath: explicit})
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
	home := t.TempDir()
	r := &Resolved{
		merged: File{
			Clawbridge: cbconfig.Config{
				Clients: []cbconfig.ClientConfig{{ID: "x", Driver: "noop", Enabled: true}},
			},
		},
		home: home,
	}
	cb, err := r.ClawbridgeConfigForRun()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, memory.DotDir, "media")
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
	home := t.TempDir()
	ur := filepath.Join(home, memory.DotDir)
	f := File{}
	r := &Resolved{merged: f, home: home}
	tp, wp := r.SessionTranscriptPaths("abc123")
	wantT := filepath.Join(ur, "sessions", "abc123", "transcript.json")
	wantW := filepath.Join(ur, "sessions", "abc123", "working_transcript.json")
	if tp != wantT || wp != wantW {
		t.Fatalf("got transcript=%q working=%q", tp, wp)
	}
	f.Features.DisableTranscript = boolPtr(true)
	r2 := &Resolved{merged: f, home: home}
	tp2, wp2 := r2.SessionTranscriptPaths("abc123")
	if tp2 != "" || wp2 != "" {
		t.Fatalf("disabled transcript: got %q %q", tp2, wp2)
	}
}

func TestSessionWorkerCount(t *testing.T) {
	r := &Resolved{merged: File{}}
	if r.SessionWorkerCount() != 0 {
		t.Fatalf("unset: %d", r.SessionWorkerCount())
	}
	f := File{}
	f.Sessions.WorkerCount = 16
	r2 := &Resolved{merged: f}
	if r2.SessionWorkerCount() != 16 {
		t.Fatalf("got %d", r2.SessionWorkerCount())
	}
}

func TestSessionsSQLitePath(t *testing.T) {
	home := t.TempDir()
	ur := filepath.Join(home, memory.DotDir)
	f := File{}
	r := &Resolved{merged: f, home: home}
	got := r.SessionsSQLitePath()
	want := filepath.Join(ur, "sessions.sqlite")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	f.Sessions.DisableSQLite = boolPtr(true)
	r2 := &Resolved{merged: f, home: home}
	if r2.SessionsSQLitePath() != "" {
		t.Fatal("expected empty when disabled")
	}
	f = File{}
	f.Sessions.SQLitePath = "custom.db"
	r3 := &Resolved{merged: f, home: home}
	if r3.SessionsSQLitePath() != filepath.Join(ur, "custom.db") {
		t.Fatalf("relative: %q", r3.SessionsSQLitePath())
	}
}

func TestPushRuntime_MemoryRecallBackend(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	f := File{}
	f.Memory.Recall.Backend = "sqlite"
	r := &Resolved{merged: f}
	r.PushRuntime()
	if got := rtopts.Current().MemoryRecallBackend; got != "sqlite" {
		t.Fatalf("MemoryRecallBackend = %q, want sqlite", got)
	}
}

func TestPushRuntime_MemoryRecallSQLitePath(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })
	f := File{}
	f.Memory.Recall.SQLitePath = "memory/custom-recall.sqlite"
	r := &Resolved{merged: f}
	r.PushRuntime()
	if got := rtopts.Current().MemoryRecallSQLitePath; got != "memory/custom-recall.sqlite" {
		t.Fatalf("MemoryRecallSQLitePath = %q, want %q", got, "memory/custom-recall.sqlite")
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

func TestResolveLogPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	if ResolveLogPath(tmp, "") != "" {
		t.Fatal("empty path")
	}
	absMark, err := filepath.Abs(filepath.Join(tmp, "abs_only.log"))
	if err != nil {
		t.Fatal(err)
	}
	if got := ResolveLogPath(tmp, absMark); got != absMark {
		t.Fatalf("abs: got %q want %q", got, absMark)
	}
	rel := filepath.Join("d", "e.log")
	want := filepath.Join(tmp, rel)
	if got := ResolveLogPath(tmp, rel); got != want {
		t.Fatalf("relative: got %q want %q", got, want)
	}
}

func TestResolved_LogFile(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "config.yaml"), []byte(`
log:
  file: "run.log"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(LoadOptions{Home: home})
	if err != nil {
		t.Fatal(err)
	}
	got := r.LogFile("")
	ur := r.UserDataRoot()
	want := filepath.Join(ur, "run.log")
	if got != want {
		t.Fatalf("LogFile: got %q want %q", got, want)
	}
	ov, err := filepath.Abs(filepath.Join(ur, "override.log"))
	if err != nil {
		t.Fatal(err)
	}
	if r.LogFile(ov) != ov {
		t.Fatalf("cli override: %q", r.LogFile(ov))
	}
}

func TestLoad_explicitRelativeUnderUserDotDir(t *testing.T) {
	home := t.TempDir()
	ud := userConfigDir(home)
	if err := os.MkdirAll(ud, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ud, "layer.yaml"), []byte(`model: from-relative`), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := Load(LoadOptions{Home: home, ExplicitPath: "layer.yaml"})
	if err != nil {
		t.Fatal(err)
	}
	if r.ChatModel() != "from-relative" {
		t.Fatalf("model: %q", r.ChatModel())
	}
}
