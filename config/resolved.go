package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/openai/openai-go/option"
)

// Resolved is the merged YAML plus cwd; accessors combine file values with environment (see docs/config.md).
type Resolved struct {
	merged         File
	cwd            string
	explicitConfig string
}

// CWD returns the absolute working directory passed to Load.
func (r *Resolved) CWD() string { return r.cwd }

// ExplicitPath returns the raw --config argument, if any.
func (r *Resolved) ExplicitPath() string { return r.explicitConfig }

// HasAPIKey reports whether a non-empty API key is available (merged file or OPENAI_API_KEY).
func (r *Resolved) HasAPIKey() bool { return strings.TrimSpace(r.apiKeyResolved()) != "" }

func (r *Resolved) apiKeyResolved() string {
	if k := strings.TrimSpace(r.merged.OpenAI.APIKey); k != "" {
		return k
	}
	return strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
}

func (r *Resolved) baseURLResolved() string {
	if u := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL")); u != "" {
		return u
	}
	return strings.TrimSpace(r.merged.OpenAI.BaseURL)
}

func (r *Resolved) orgResolved() string {
	if o := strings.TrimSpace(os.Getenv("OPENAI_ORG_ID")); o != "" {
		return o
	}
	return strings.TrimSpace(r.merged.OpenAI.OrgID)
}

func (r *Resolved) projectResolved() string {
	if p := strings.TrimSpace(os.Getenv("OPENAI_PROJECT_ID")); p != "" {
		return p
	}
	return strings.TrimSpace(r.merged.OpenAI.ProjectID)
}

// OpenAIOptions returns client options so secrets need not be copied into process environment.
// File api_key wins over OPENAI_API_KEY when set in YAML; base URL and org/project allow env override.
func (r *Resolved) OpenAIOptions() []option.RequestOption {
	var opts []option.RequestOption
	if k := r.apiKeyResolved(); k != "" {
		opts = append(opts, option.WithAPIKey(k))
	}
	if u := r.baseURLResolved(); u != "" {
		opts = append(opts, option.WithBaseURL(u))
	}
	if o := r.orgResolved(); o != "" {
		opts = append(opts, option.WithOrganization(o))
	}
	if p := r.projectResolved(); p != "" {
		opts = append(opts, option.WithProject(p))
	}
	return opts
}

// ChatModel returns the chat model: ONCLAW_MODEL if set, else YAML model, else empty (caller keeps default).
func (r *Resolved) ChatModel() string {
	if m := strings.TrimSpace(os.Getenv("ONCLAW_MODEL")); m != "" {
		return m
	}
	return strings.TrimSpace(r.merged.Model)
}

// Channels returns merged `channels:` entries. Empty means callers should fall back to “start all registered specs”.
func (r *Resolved) Channels() []ChannelConfig {
	if r == nil {
		return nil
	}
	return r.merged.Channels
}

// ChatTransport returns ONCLAW_CHAT_TRANSPORT if set, else YAML chat.transport, else empty (use library default / env).
func (r *Resolved) ChatTransport() string {
	if t := strings.TrimSpace(os.Getenv("ONCLAW_CHAT_TRANSPORT")); t != "" {
		return t
	}
	return strings.TrimSpace(r.merged.Chat.Transport)
}

// LogLevel returns CLI/env override first, then YAML log.level.
func (r *Resolved) LogLevel(cliOverride string) string {
	if v := strings.TrimSpace(cliOverride); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("ONCLAW_LOG_LEVEL")); v != "" {
		return v
	}
	return strings.TrimSpace(r.merged.Log.Level)
}

// LogFormat returns CLI/env override first, then YAML log.format.
func (r *Resolved) LogFormat(cliOverride string) string {
	if v := strings.TrimSpace(cliOverride); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("ONCLAW_LOG_FORMAT")); v != "" {
		return v
	}
	return strings.TrimSpace(r.merged.Log.Format)
}

// TranscriptPath mirrors cmd/oneclaw resolveTranscriptPath using env, YAML, and defaults.
func (r *Resolved) TranscriptPath() string {
	if r.transcriptDisabled() {
		return ""
	}
	p := strings.TrimSpace(os.Getenv("ONCLAW_TRANSCRIPT_PATH"))
	if p == "" {
		p = strings.TrimSpace(r.merged.Paths.Transcript)
	}
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

func (r *Resolved) transcriptDisabled() bool {
	if v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_TRANSCRIPT")); v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
		return true
	}
	if r.merged.Features.DisableTranscript != nil && *r.merged.Features.DisableTranscript {
		return true
	}
	return false
}

// MaintainLoopInterval parses maintain.interval / ONCLAW_MAINTAIN_INTERVAL (env wins).
func (r *Resolved) MaintainLoopInterval() time.Duration {
	v := strings.TrimSpace(os.Getenv("ONCLAW_MAINTAIN_INTERVAL"))
	if v == "" {
		v = strings.TrimSpace(r.merged.Maintain.Interval)
	}
	if v == "" {
		return time.Hour
	}
	if v == "0" || strings.EqualFold(v, "off") || strings.EqualFold(v, "false") {
		return 0
	}
	d, err := time.ParseDuration(v)
	if err != nil || d < 0 {
		slog.Warn("config.invalid_maintain_interval", "ONCLAW_MAINTAIN_INTERVAL", v, "fallback", "1h")
		return time.Hour
	}
	return d
}

// MaintainCronSpec returns an optional cron expression for cmd/maintain when using cron mode instead of a fixed interval.
// Standard 5-field spec (minute hour day-of-month month day-of-week) plus descriptors such as @every 1h (robfig/cron v3).
// ONCLAW_MAINTAIN_CRON wins over maintain.cron in YAML when non-empty.
func (r *Resolved) MaintainCronSpec() string {
	if v := strings.TrimSpace(os.Getenv("ONCLAW_MAINTAIN_CRON")); v != "" {
		return v
	}
	return strings.TrimSpace(r.merged.Maintain.Cron)
}

// ApplyEnvDefaults sets ONCLAW_* (never OPENAI_API_KEY) when the variable is unset, so existing packages keep working.
func ApplyEnvDefaults(r *Resolved) {
	if r == nil {
		return
	}
	setIfEmpty("ONCLAW_MEMORY_BASE", strings.TrimSpace(r.merged.Paths.MemoryBase))
	setIfEmpty("ONCLAW_TRANSCRIPT_PATH", strings.TrimSpace(r.merged.Paths.Transcript))
	setIfEmpty("ONCLAW_TURN_LOG_PATH", strings.TrimSpace(r.merged.Paths.TurnLogPath))
	setIfEmpty("ONCLAW_MODEL", strings.TrimSpace(r.merged.Model))
	setIfEmpty("ONCLAW_CHAT_TRANSPORT", strings.TrimSpace(r.merged.Chat.Transport))

	if r.merged.Budget.MaxPromptBytes != 0 {
		setIfEmpty("ONCLAW_MAX_PROMPT_BYTES", strconv.Itoa(r.merged.Budget.MaxPromptBytes))
	}
	if r.merged.Budget.MinTranscriptMessages != 0 {
		setIfEmpty("ONCLAW_MIN_TRANSCRIPT_MESSAGES", strconv.Itoa(r.merged.Budget.MinTranscriptMessages))
	}
	if r.merged.Budget.RecallMaxBytes != 0 {
		setIfEmpty("ONCLAW_RECALL_MAX_BYTES", strconv.Itoa(r.merged.Budget.RecallMaxBytes))
	}

	if r.merged.Maintain.Interval != "" {
		setIfEmpty("ONCLAW_MAINTAIN_INTERVAL", strings.TrimSpace(r.merged.Maintain.Interval))
	}
	if r.merged.Maintain.Cron != "" {
		setIfEmpty("ONCLAW_MAINTAIN_CRON", strings.TrimSpace(r.merged.Maintain.Cron))
	}
	if r.merged.Maintain.Model != "" {
		setIfEmpty("ONCLAW_MAINTENANCE_MODEL", strings.TrimSpace(r.merged.Maintain.Model))
	}
	if r.merged.Maintain.ScheduledModel != "" {
		setIfEmpty("ONCLAW_MAINTENANCE_SCHEDULED_MODEL", strings.TrimSpace(r.merged.Maintain.ScheduledModel))
	}
	if r.merged.Maintain.MaxTokens != 0 {
		setIfEmpty("ONCLAW_MAINTENANCE_MAX_TOKENS", strconv.FormatInt(r.merged.Maintain.MaxTokens, 10))
	}
	if r.merged.Maintain.MinLogBytes != 0 {
		setIfEmpty("ONCLAW_MAINTENANCE_MIN_LOG_BYTES", strconv.Itoa(r.merged.Maintain.MinLogBytes))
	}
	if r.merged.Maintain.MaxLogReadBytes != 0 {
		setIfEmpty("ONCLAW_MAINTENANCE_MAX_LOG_BYTES", strconv.Itoa(r.merged.Maintain.MaxLogReadBytes))
	}

	if r.merged.Log.Level != "" {
		setIfEmpty("ONCLAW_LOG_LEVEL", strings.TrimSpace(r.merged.Log.Level))
	}
	if r.merged.Log.Format != "" {
		setIfEmpty("ONCLAW_LOG_FORMAT", strings.TrimSpace(r.merged.Log.Format))
	}

	setBoolDisable("ONCLAW_DISABLE_TRANSCRIPT", r.merged.Features.DisableTranscript)
	setBoolDisable("ONCLAW_DISABLE_MEMORY", r.merged.Features.DisableMemory)
	setBoolDisable("ONCLAW_DISABLE_TURN_LOG", r.merged.Features.DisableTurnLog)
	setBoolDisable("ONCLAW_DISABLE_AUTO_MEMORY", r.merged.Features.DisableAutoMemory)
	setBoolDisable("ONCLAW_DISABLE_MEMORY_EXTRACT", r.merged.Features.DisableMemoryExtract)
	setBoolDisable("ONCLAW_DISABLE_AUTO_MAINTENANCE", r.merged.Features.DisableAutoMaintenance)
	setBoolDisable("ONCLAW_DISABLE_MEMORY_AUDIT", r.merged.Features.DisableMemoryAudit)
	setBoolDisable("ONCLAW_DISABLE_CONTEXT_BUDGET", r.merged.Features.DisableContextBudget)
}

func setIfEmpty(key, val string) {
	if val == "" || strings.TrimSpace(os.Getenv(key)) != "" {
		return
	}
	_ = os.Setenv(key, val)
}

func setBoolDisable(key string, b *bool) {
	if b == nil || !*b {
		return
	}
	if strings.TrimSpace(os.Getenv(key)) != "" {
		return
	}
	_ = os.Setenv(key, "1")
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
