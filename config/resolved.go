package config

import (
	"fmt"
	"path/filepath"
	"strings"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"github.com/lengzhao/oneclaw/workspace"
	"github.com/openai/openai-go/option"
)

func expandTilde(home, p string) string {
	if home == "" || p == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if len(p) >= 2 && p[0] == '~' {
		sep := p[1]
		if sep == filepath.Separator || sep == '/' || sep == '\\' {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// Resolved is the merged YAML plus home; accessors read file values only (see docs/config.md).
type Resolved struct {
	merged         File
	home           string
	explicitConfig string
}

// UserDataRoot is the host data directory (~/.oneclaw or paths.memory_base): sessions, transcripts, and flat host files.
func (r *Resolved) UserDataRoot() string {
	if r == nil || r.home == "" {
		return ""
	}
	mb := strings.TrimSpace(r.merged.Paths.MemoryBase)
	if mb != "" {
		return filepath.Clean(expandTilde(r.home, mb))
	}
	return filepath.Join(r.home, workspace.DotDir)
}

// HasAPIKey reports whether a non-empty API key is set in merged YAML.
func (r *Resolved) HasAPIKey() bool { return strings.TrimSpace(r.merged.OpenAI.APIKey) != "" }

func (r *Resolved) apiKeyResolved() string {
	return strings.TrimSpace(r.merged.OpenAI.APIKey)
}

// OpenAIAPIKey returns merged openai.api_key (trimmed).
func (r *Resolved) OpenAIAPIKey() string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.merged.OpenAI.APIKey)
}

// OpenAIBaseURL returns merged openai.base_url (trimmed).
func (r *Resolved) OpenAIBaseURL() string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.merged.OpenAI.BaseURL)
}

// OpenAIOptions returns client options from merged YAML (api_key, base_url, org, project).
func (r *Resolved) OpenAIOptions() []option.RequestOption {
	var opts []option.RequestOption
	if k := r.apiKeyResolved(); k != "" {
		opts = append(opts, option.WithAPIKey(k))
	}
	if u := strings.TrimSpace(r.merged.OpenAI.BaseURL); u != "" {
		opts = append(opts, option.WithBaseURL(u))
	}
	if o := strings.TrimSpace(r.merged.OpenAI.OrgID); o != "" {
		opts = append(opts, option.WithOrganization(o))
	}
	if p := strings.TrimSpace(r.merged.OpenAI.ProjectID); p != "" {
		opts = append(opts, option.WithProject(p))
	}
	return opts
}

// ChatModel returns YAML model, else empty (caller keeps default).
func (r *Resolved) ChatModel() string {
	return strings.TrimSpace(r.merged.Model)
}

const (
	defaultMainAgentMaxSteps = 100
	minMainAgentMaxSteps     = 1
	maxMainAgentMaxSteps     = 256

	defaultMainAgentMaxCompletionTokens int64 = 32768
	minMainAgentMaxCompletionTokens     int64 = 1024
	maxMainAgentMaxCompletionTokens     int64 = 131072
)

// MainAgentMaxSteps returns agent.max_steps from YAML, clamped to [1, 256]; 0/unset → defaultMainAgentMaxSteps (100).
func (r *Resolved) MainAgentMaxSteps() int {
	if r == nil {
		return defaultMainAgentMaxSteps
	}
	n := r.merged.Agent.MaxSteps
	if n <= 0 {
		return defaultMainAgentMaxSteps
	}
	if n < minMainAgentMaxSteps {
		n = minMainAgentMaxSteps
	}
	if n > maxMainAgentMaxSteps {
		n = maxMainAgentMaxSteps
	}
	return n
}

// MainAgentMaxCompletionTokens returns agent.max_tokens for the main chat loop (API max_completion_tokens per step).
// 0/unset in YAML yields defaultMainAgentMaxCompletionTokens; values are clamped to
// [minMainAgentMaxCompletionTokens, maxMainAgentMaxCompletionTokens].
func (r *Resolved) MainAgentMaxCompletionTokens() int64 {
	if r == nil {
		return defaultMainAgentMaxCompletionTokens
	}
	t := r.merged.Agent.MaxTokens
	if t <= 0 {
		return defaultMainAgentMaxCompletionTokens
	}
	if t < minMainAgentMaxCompletionTokens {
		t = minMainAgentMaxCompletionTokens
	}
	if t > maxMainAgentMaxCompletionTokens {
		t = maxMainAgentMaxCompletionTokens
	}
	return t
}

// ClawbridgeConfigForRun returns merged clawbridge config for clawbridge.New.
// When media.root is empty in YAML, it defaults to <UserDataRoot>/media.
func (r *Resolved) ClawbridgeConfigForRun() (cbconfig.Config, error) {
	if r == nil {
		return cbconfig.Config{}, fmt.Errorf("config: nil resolved")
	}
	cfg := r.merged.Clawbridge
	if strings.TrimSpace(cfg.Media.Root) == "" {
		cfg.Media.Root = filepath.Join(r.UserDataRoot(), "media")
	}
	return cfg, nil
}

// ChatTransport returns YAML chat.transport, else empty (use library default).
func (r *Resolved) ChatTransport() string {
	return strings.TrimSpace(r.merged.Chat.Transport)
}

// LogLevel returns CLI override first, then YAML log.level.
func (r *Resolved) LogLevel(cliOverride string) string {
	if v := strings.TrimSpace(cliOverride); v != "" {
		return v
	}
	return strings.TrimSpace(r.merged.Log.Level)
}

// LogFormat returns CLI override first, then YAML log.format.
func (r *Resolved) LogFormat(cliOverride string) string {
	if v := strings.TrimSpace(cliOverride); v != "" {
		return v
	}
	return strings.TrimSpace(r.merged.Log.Format)
}

// ResolveLogPath turns a config or CLI log path into an absolute path; empty p returns "".
func ResolveLogPath(cwd, p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(filepath.Join(cwd, p))
	if err != nil {
		return filepath.Join(cwd, p)
	}
	return abs
}

// LogFile returns an absolute log file path: CLI override wins, then YAML log.file.
// Empty string means file logging is disabled. Relative paths resolve from UserDataRoot.
func (r *Resolved) LogFile(cliOverride string) string {
	base := r.UserDataRoot()
	if v := strings.TrimSpace(cliOverride); v != "" {
		return ResolveLogPath(base, v)
	}
	return ResolveLogPath(base, r.merged.Log.File)
}

// WorkingTranscriptMaxMessages caps how many tail messages are written to working_transcript.json
// (after user-visible collapse). 0 means default 30. Negative means no limit.
func (r *Resolved) WorkingTranscriptMaxMessages() int {
	if r == nil {
		return 0
	}
	return r.merged.Paths.WorkingTranscriptMaxMessages
}

func (r *Resolved) transcriptDisabled() bool {
	return boolPtrTrue(r.merged.Features.DisableTranscript)
}

// SessionTranscriptDir is the session workspace root: <userDataRoot>/sessions/<id>/.
func (r *Resolved) SessionTranscriptDir(sessionSegment string) string {
	seg := strings.TrimSpace(sessionSegment)
	if seg == "" {
		seg = "default"
	}
	return filepath.Join(r.UserDataRoot(), "sessions", seg)
}

// SessionWorkerCount returns sessions.worker_count; values < 1 mean the session package default (8).
func (r *Resolved) SessionWorkerCount() int {
	if r == nil {
		return 0
	}
	return r.merged.Sessions.WorkerCount
}

// SessionIsolateWorkspace reports sessions.isolate_workspace (default false: shared UserDataRoot as Engine.CWD).
func (r *Resolved) SessionIsolateWorkspace() bool {
	if r == nil {
		return false
	}
	return boolPtrTrue(r.merged.Sessions.IsolateWorkspace)
}

// SessionTurnPolicyRaw returns sessions.turn_policy from YAML (trimmed). Empty means serial; see session.ParseTurnPolicy.
func (r *Resolved) SessionTurnPolicyRaw() string {
	if r == nil {
		return ""
	}
	return strings.TrimSpace(r.merged.Sessions.TurnPolicy)
}

// SessionTranscriptPaths returns per-session transcript.json and working_transcript.json paths.
// When transcript is disabled, both are empty.
func (r *Resolved) SessionTranscriptPaths(sessionSegment string) (transcript, working string) {
	if r.transcriptDisabled() {
		return "", ""
	}
	dir := r.SessionTranscriptDir(sessionSegment)
	return filepath.Join(dir, "transcript.json"), filepath.Join(dir, "working_transcript.json")
}

// MultimodalImageDisabled reports features.disable_multimodal_image (default false = vision parts allowed).
func (r *Resolved) MultimodalImageDisabled() bool {
	if r == nil {
		return false
	}
	return boolPtrTrue(r.merged.Features.DisableMultimodalImage)
}

// MultimodalAudioDisabled reports features.disable_multimodal_audio (default false = input_audio allowed for wav/mp3).
func (r *Resolved) MultimodalAudioDisabled() bool {
	if r == nil {
		return false
	}
	return boolPtrTrue(r.merged.Features.DisableMultimodalAudio)
}

func boolPtrTrue(p *bool) bool {
	return p != nil && *p
}
