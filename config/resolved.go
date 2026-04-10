package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/openai/openai-go/option"
)

// Resolved is the merged YAML plus cwd; accessors read file values only (see docs/config.md).
type Resolved struct {
	merged         File
	cwd            string
	explicitConfig string
}

// CWD returns the absolute working directory passed to Load.
func (r *Resolved) CWD() string { return r.cwd }

// ExplicitPath returns the raw --config argument, if any.
func (r *Resolved) ExplicitPath() string { return r.explicitConfig }

// HasAPIKey reports whether a non-empty API key is set in merged YAML.
func (r *Resolved) HasAPIKey() bool { return strings.TrimSpace(r.merged.OpenAI.APIKey) != "" }

func (r *Resolved) apiKeyResolved() string {
	return strings.TrimSpace(r.merged.OpenAI.APIKey)
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
// When media.root is empty in YAML, it defaults to <cwd>/.oneclaw/media.
func (r *Resolved) ClawbridgeConfigForRun() (cbconfig.Config, error) {
	if r == nil {
		return cbconfig.Config{}, fmt.Errorf("config: nil resolved")
	}
	cfg := r.merged.Clawbridge
	if strings.TrimSpace(cfg.Media.Root) == "" {
		cfg.Media.Root = filepath.Join(r.cwd, memory.DotDir, "media")
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

// TranscriptPath resolves transcript file path from YAML and defaults.
func (r *Resolved) TranscriptPath() string {
	if r.transcriptDisabled() {
		return ""
	}
	p := strings.TrimSpace(r.merged.Paths.Transcript)
	if p == "" {
		return filepath.Join(r.cwd, memory.DotDir, "transcript.json")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(filepath.Join(r.cwd, p))
	if err != nil {
		return filepath.Join(r.cwd, p)
	}
	return abs
}

// WorkingTranscriptMaxMessages caps how many tail messages are written to working_transcript.json
// (after user-visible collapse). 0 means default 30. Negative means no limit.
func (r *Resolved) WorkingTranscriptMaxMessages() int {
	if r == nil {
		return 0
	}
	return r.merged.Paths.WorkingTranscriptMaxMessages
}

// WorkingTranscriptPath persists the user-visible message list (same shape as in-memory Messages after each turn).
// When transcript is disabled, returns empty. Default: <cwd>/.oneclaw/working_transcript.json
func (r *Resolved) WorkingTranscriptPath() string {
	if r.transcriptDisabled() {
		return ""
	}
	p := strings.TrimSpace(r.merged.Paths.WorkingTranscript)
	if p == "" {
		return filepath.Join(r.cwd, memory.DotDir, "working_transcript.json")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(filepath.Join(r.cwd, p))
	if err != nil {
		return filepath.Join(r.cwd, p)
	}
	return abs
}

func (r *Resolved) transcriptDisabled() bool {
	return boolPtrTrue(r.merged.Features.DisableTranscript)
}

// SessionsSQLiteDisabled reports sessions.disable_sqlite (default false = SQLite enabled when path resolves).
func (r *Resolved) SessionsSQLiteDisabled() bool {
	if r == nil {
		return true
	}
	return boolPtrTrue(r.merged.Sessions.DisableSQLite)
}

// SessionsSQLitePath returns the SQLite database path for session metadata and recall state.
// Empty means "do not open SQLite" (when disabled or misconfigured).
func (r *Resolved) SessionsSQLitePath() string {
	if r == nil || r.SessionsSQLiteDisabled() {
		return ""
	}
	p := strings.TrimSpace(r.merged.Sessions.SQLitePath)
	if p == "" {
		return filepath.Join(r.cwd, memory.DotDir, "sessions.sqlite")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	abs, err := filepath.Abs(filepath.Join(r.cwd, p))
	if err != nil {
		return filepath.Join(r.cwd, p)
	}
	return abs
}

// SessionTranscriptDir is the directory for one logical session's transcript files (under .oneclaw/sessions/).
func (r *Resolved) SessionTranscriptDir(sessionSegment string) string {
	seg := strings.TrimSpace(sessionSegment)
	if seg == "" {
		seg = "default"
	}
	return filepath.Join(r.cwd, memory.DotDir, "sessions", seg)
}

// SessionWorkerCount returns sessions.worker_count; values < 1 mean the session package default (8).
func (r *Resolved) SessionWorkerCount() int {
	if r == nil {
		return 0
	}
	return r.merged.Sessions.WorkerCount
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

// NotifyAuditSinkPaths returns which notify audit JSONL sinks should register.
// Default is all true (llm, orchestration, visible). disable_audit_sinks disables all;
// per-path flags further turn off individual sinks.
func (r *Resolved) NotifyAuditSinkPaths() (llm, orchestration, visible bool) {
	if r == nil {
		return true, true, true
	}
	if boolPtrTrue(r.merged.Features.DisableAuditSinks) {
		return false, false, false
	}
	return !boolPtrTrue(r.merged.Features.DisableAuditLLM),
		!boolPtrTrue(r.merged.Features.DisableAuditOrchestration),
		!boolPtrTrue(r.merged.Features.DisableAuditVisible)
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

// EmbeddedScheduledMaintainInterval returns the interval for in-process maintainloop (oneclaw main).
// It is 0 unless maintain.interval is non-empty in merged YAML.
func (r *Resolved) EmbeddedScheduledMaintainInterval() time.Duration {
	if strings.TrimSpace(r.merged.Maintain.Interval) == "" {
		return 0
	}
	return r.MaintainLoopInterval()
}

// MaintainLoopInterval parses maintain.interval from YAML.
func (r *Resolved) MaintainLoopInterval() time.Duration {
	v := strings.TrimSpace(r.merged.Maintain.Interval)
	if v == "" {
		return time.Hour
	}
	if v == "0" || strings.EqualFold(v, "off") || strings.EqualFold(v, "false") {
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		slog.Warn("config.invalid_maintain_interval", "maintain.interval", v, "fallback", "1h")
		return time.Hour
	}
	return d
}

// ValidateUserConfigPath returns an error if home-relative config exists but is not readable YAML (optional helper).
func ValidateUserConfigPath(home string) error {
	p := filepath.Join(home, UserRelPath)
	if _, err := os.Stat(p); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	_, err := readFileLayer(p)
	if err != nil {
		return fmt.Errorf("config: %s: %w", p, err)
	}
	return nil
}
