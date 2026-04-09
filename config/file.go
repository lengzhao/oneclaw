// Package config loads oneclaw YAML configuration with a single merge rule for dev and production.
package config

import (
	"os"
	"path/filepath"
	"strings"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"github.com/lengzhao/oneclaw/memory"
	"gopkg.in/yaml.v3"
)

// UserRelPath is the config file under the user's home directory (~/.oneclaw/config.yaml).
const UserRelPath = ".oneclaw/config.yaml"

// File is one YAML layer. Empty strings mean "not set" for merge purposes.
type File struct {
	OpenAI OpenAIFile `yaml:"openai"`
	Model  string     `yaml:"model"`
	// Agent configures the main session model loop (per inbound turn).
	Agent struct {
		// MaxSteps is model calls per turn; the last call is sent without tools (earlier calls can use tools).
		MaxSteps int `yaml:"max_steps"`
	} `yaml:"agent"`

	Chat struct {
		Transport string `yaml:"transport"`
	} `yaml:"chat"`

	Paths struct {
		MemoryBase                   string `yaml:"memory_base"`
		Transcript                   string `yaml:"transcript"`
		WorkingTranscript            string `yaml:"working_transcript"`
		WorkingTranscriptMaxMessages int    `yaml:"working_transcript_max_messages"` // 0=default 30; <0=unlimited
	} `yaml:"paths"`

	Features struct {
		DisableTranscript           *bool `yaml:"disable_transcript"`
		DisableMemory               *bool `yaml:"disable_memory"`
		DisableAutoMemory           *bool `yaml:"disable_auto_memory"`
		DisableMemoryExtract        *bool `yaml:"disable_memory_extract"`
		DisableAutoMaintenance      *bool `yaml:"disable_auto_maintenance"`
		DisableScheduledMaintenance *bool `yaml:"disable_scheduled_maintenance"`
		DisableMemoryAudit          *bool `yaml:"disable_memory_audit"`
		DisableContextBudget        *bool `yaml:"disable_context_budget"`
		DisableUsageLedger          *bool `yaml:"disable_usage_ledger"`
		UsageEstimateCost           *bool `yaml:"usage_estimate_cost"`
		DisableBehaviorPolicyWrite  *bool `yaml:"disable_behavior_policy_write"`
		DisableScheduledTasks       *bool `yaml:"disable_scheduled_tasks"`
		DisableSemanticCompact      *bool `yaml:"disable_semantic_compact"`
		DisableSkills               *bool `yaml:"disable_skills"`
		DisableTasks                *bool `yaml:"disable_tasks"`
		// Notify audit JSONL sinks (.oneclaw/audit/...): disable_audit_sinks turns all off; the rest are per-path.
		DisableAuditSinks           *bool `yaml:"disable_audit_sinks"`
		DisableAuditLLM             *bool `yaml:"disable_audit_llm"`
		DisableAuditOrchestration   *bool `yaml:"disable_audit_orchestration"`
		DisableAuditVisible         *bool `yaml:"disable_audit_visible"`
		// DisableMultimodalImage: when true, inbound images use read_file hints only (no vision API parts).
		DisableMultimodalImage *bool `yaml:"disable_multimodal_image"`
		// DisableMultimodalAudio: when true, inbound wav/mp3 use read_file hints only (no input_audio parts).
		DisableMultimodalAudio *bool `yaml:"disable_multimodal_audio"`
	} `yaml:"features"`

	Budget struct {
		MaxContextTokens      int `yaml:"max_context_tokens"` // ×2 → byte budget when max_prompt_bytes unset
		MaxPromptBytes        int `yaml:"max_prompt_bytes"`
		MinTranscriptMessages int `yaml:"min_transcript_messages"`
		RecallMaxBytes        int `yaml:"recall_max_bytes"`
		HistoryMaxBytes       int `yaml:"history_max_bytes"`
		SystemExtraMaxBytes   int `yaml:"system_extra_max_bytes"`
		AgentMdMaxBytes       int `yaml:"agent_md_max_bytes"`
		SkillIndexMaxBytes    int `yaml:"skill_index_max_bytes"`
		InheritedMessages     int `yaml:"inherited_messages"`
	} `yaml:"budget"`

	Maintain struct {
		Interval        string `yaml:"interval"`
		LogDays         int    `yaml:"log_days"` // calendar-mode window for scheduled maintain; 0 = default 3
		Model           string `yaml:"model"`
		ScheduledModel  string `yaml:"scheduled_model"`
		MaxTokens       int64  `yaml:"max_tokens"`
		MinLogBytes     int    `yaml:"min_log_bytes"`
		MaxLogReadBytes int    `yaml:"max_log_bytes"`
		PostTurn        struct {
			LogDays                int   `yaml:"log_days"`
			MaxCombinedLogBytes    int   `yaml:"max_combined_log_bytes"`
			MaxLogBytes            int   `yaml:"max_log_bytes"`
			MinLogBytes            int   `yaml:"min_log_bytes"`
			MaxTopicFiles          int   `yaml:"max_topic_files"`
			TopicExcerptBytes      int   `yaml:"topic_excerpt_bytes"`
			MemoryPreviewBytes     int   `yaml:"memory_preview_bytes"`
			TimeoutSeconds         int   `yaml:"timeout_seconds"`
			MaxTokens              int64 `yaml:"max_tokens"`
			UserSnapshotBytes      int   `yaml:"user_snapshot_bytes"`
			AssistantSnapshotBytes int   `yaml:"assistant_snapshot_bytes"`
		} `yaml:"post_turn"`
		ScheduledTimeoutSeconds int    `yaml:"scheduled_timeout_seconds"`
		ScheduledMaxSteps       int    `yaml:"scheduled_max_steps"`
		IncrementalOverlap      string `yaml:"incremental_overlap"`
		IncrementalMaxSpan      string `yaml:"incremental_max_span"`
	} `yaml:"maintain"`

	Log struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"log"`

	SidechainMerge string `yaml:"sidechain_merge"`

	Usage struct {
		DefaultInputPerMtok  float64 `yaml:"default_input_per_mtok"`
		DefaultOutputPerMtok float64 `yaml:"default_output_per_mtok"`
	} `yaml:"usage"`

	Schedule struct {
		MinSleep  string `yaml:"min_sleep"`
		IdleSleep string `yaml:"idle_sleep"`
	} `yaml:"schedule"`

	SemanticCompact struct {
		SummaryMaxBytes int `yaml:"summary_max_bytes"`
	} `yaml:"semantic_compact"`

	Skills struct {
		RecentPath string `yaml:"recent_path"`
	} `yaml:"skills"`

	// Sessions: per-chat isolation (IM threads). SQLite stores session index + recall state; transcripts stay as files under .oneclaw/sessions/<id>/.
	Sessions struct {
		DisableSQLite *bool  `yaml:"disable_sqlite"`
		SQLitePath    string `yaml:"sqlite_path"`
		// WorkerCount: fixed goroutine shards for cmd/oneclaw (hash session → worker). 0 = default 8.
		WorkerCount int `yaml:"worker_count"`
	} `yaml:"sessions"`

	// Clawbridge is IM 总线配置，形状与 github.com/lengzhao/clawbridge/config.Config 一致（media + clients）。
	// 见 clawbridge 仓库 config.example.yaml。media.root 留空时运行时默认为 <cwd>/.oneclaw/media。
	Clawbridge cbconfig.Config `yaml:"clawbridge"`

	MCP MCPFile `yaml:"mcp"`
}

// OpenAIFile holds OpenAI-compatible client settings. api_key is sensitive; keep in file, not in process env.
type OpenAIFile struct {
	APIKey    string `yaml:"api_key"`
	BaseURL   string `yaml:"base_url"`
	OrgID     string `yaml:"org_id"`
	ProjectID string `yaml:"project_id"`
}

func readFileLayer(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}
	var f File
	if err := yaml.Unmarshal(b, &f); err != nil {
		return File{}, err
	}
	return f, nil
}

func mergeFile(dst *File, src File) {
	if src.OpenAI.APIKey != "" {
		dst.OpenAI.APIKey = src.OpenAI.APIKey
	}
	if src.OpenAI.BaseURL != "" {
		dst.OpenAI.BaseURL = src.OpenAI.BaseURL
	}
	if src.OpenAI.OrgID != "" {
		dst.OpenAI.OrgID = src.OpenAI.OrgID
	}
	if src.OpenAI.ProjectID != "" {
		dst.OpenAI.ProjectID = src.OpenAI.ProjectID
	}
	if src.Model != "" {
		dst.Model = src.Model
	}
	if src.Agent.MaxSteps != 0 {
		dst.Agent.MaxSteps = src.Agent.MaxSteps
	}
	if src.Chat.Transport != "" {
		dst.Chat.Transport = src.Chat.Transport
	}
	if src.Paths.MemoryBase != "" {
		dst.Paths.MemoryBase = src.Paths.MemoryBase
	}
	if src.Paths.Transcript != "" {
		dst.Paths.Transcript = src.Paths.Transcript
	}
	if src.Paths.WorkingTranscript != "" {
		dst.Paths.WorkingTranscript = src.Paths.WorkingTranscript
	}
	if src.Paths.WorkingTranscriptMaxMessages != 0 {
		dst.Paths.WorkingTranscriptMaxMessages = src.Paths.WorkingTranscriptMaxMessages
	}
	mergeBoolPtr(&dst.Features.DisableTranscript, src.Features.DisableTranscript)
	mergeBoolPtr(&dst.Features.DisableMemory, src.Features.DisableMemory)
	mergeBoolPtr(&dst.Features.DisableAutoMemory, src.Features.DisableAutoMemory)
	mergeBoolPtr(&dst.Features.DisableMemoryExtract, src.Features.DisableMemoryExtract)
	mergeBoolPtr(&dst.Features.DisableAutoMaintenance, src.Features.DisableAutoMaintenance)
	mergeBoolPtr(&dst.Features.DisableScheduledMaintenance, src.Features.DisableScheduledMaintenance)
	mergeBoolPtr(&dst.Features.DisableMemoryAudit, src.Features.DisableMemoryAudit)
	mergeBoolPtr(&dst.Features.DisableContextBudget, src.Features.DisableContextBudget)
	mergeBoolPtr(&dst.Features.DisableUsageLedger, src.Features.DisableUsageLedger)
	mergeBoolPtr(&dst.Features.UsageEstimateCost, src.Features.UsageEstimateCost)
	mergeBoolPtr(&dst.Features.DisableBehaviorPolicyWrite, src.Features.DisableBehaviorPolicyWrite)
	mergeBoolPtr(&dst.Features.DisableScheduledTasks, src.Features.DisableScheduledTasks)
	mergeBoolPtr(&dst.Features.DisableSemanticCompact, src.Features.DisableSemanticCompact)
	mergeBoolPtr(&dst.Features.DisableSkills, src.Features.DisableSkills)
	mergeBoolPtr(&dst.Features.DisableTasks, src.Features.DisableTasks)
	mergeBoolPtr(&dst.Features.DisableAuditSinks, src.Features.DisableAuditSinks)
	mergeBoolPtr(&dst.Features.DisableAuditLLM, src.Features.DisableAuditLLM)
	mergeBoolPtr(&dst.Features.DisableAuditOrchestration, src.Features.DisableAuditOrchestration)
	mergeBoolPtr(&dst.Features.DisableAuditVisible, src.Features.DisableAuditVisible)
	mergeBoolPtr(&dst.Features.DisableMultimodalImage, src.Features.DisableMultimodalImage)
	mergeBoolPtr(&dst.Features.DisableMultimodalAudio, src.Features.DisableMultimodalAudio)
	if src.Budget.MaxContextTokens != 0 {
		dst.Budget.MaxContextTokens = src.Budget.MaxContextTokens
	}
	if src.Budget.MaxPromptBytes != 0 {
		dst.Budget.MaxPromptBytes = src.Budget.MaxPromptBytes
	}
	if src.Budget.MinTranscriptMessages != 0 {
		dst.Budget.MinTranscriptMessages = src.Budget.MinTranscriptMessages
	}
	if src.Budget.RecallMaxBytes != 0 {
		dst.Budget.RecallMaxBytes = src.Budget.RecallMaxBytes
	}
	if src.Budget.HistoryMaxBytes != 0 {
		dst.Budget.HistoryMaxBytes = src.Budget.HistoryMaxBytes
	}
	if src.Budget.SystemExtraMaxBytes != 0 {
		dst.Budget.SystemExtraMaxBytes = src.Budget.SystemExtraMaxBytes
	}
	if src.Budget.AgentMdMaxBytes != 0 {
		dst.Budget.AgentMdMaxBytes = src.Budget.AgentMdMaxBytes
	}
	if src.Budget.SkillIndexMaxBytes != 0 {
		dst.Budget.SkillIndexMaxBytes = src.Budget.SkillIndexMaxBytes
	}
	if src.Budget.InheritedMessages != 0 {
		dst.Budget.InheritedMessages = src.Budget.InheritedMessages
	}
	if src.Maintain.Interval != "" {
		dst.Maintain.Interval = src.Maintain.Interval
	}
	if src.Maintain.LogDays != 0 {
		dst.Maintain.LogDays = src.Maintain.LogDays
	}
	if src.Maintain.Model != "" {
		dst.Maintain.Model = src.Maintain.Model
	}
	if src.Maintain.ScheduledModel != "" {
		dst.Maintain.ScheduledModel = src.Maintain.ScheduledModel
	}
	if src.Maintain.MaxTokens != 0 {
		dst.Maintain.MaxTokens = src.Maintain.MaxTokens
	}
	if src.Maintain.MinLogBytes != 0 {
		dst.Maintain.MinLogBytes = src.Maintain.MinLogBytes
	}
	if src.Maintain.MaxLogReadBytes != 0 {
		dst.Maintain.MaxLogReadBytes = src.Maintain.MaxLogReadBytes
	}
	spt := src.Maintain.PostTurn
	dpt := &dst.Maintain.PostTurn
	if spt.LogDays != 0 {
		dpt.LogDays = spt.LogDays
	}
	if spt.MaxCombinedLogBytes != 0 {
		dpt.MaxCombinedLogBytes = spt.MaxCombinedLogBytes
	}
	if spt.MaxLogBytes != 0 {
		dpt.MaxLogBytes = spt.MaxLogBytes
	}
	if spt.MinLogBytes != 0 {
		dpt.MinLogBytes = spt.MinLogBytes
	}
	if spt.MaxTopicFiles != 0 {
		dpt.MaxTopicFiles = spt.MaxTopicFiles
	}
	if spt.TopicExcerptBytes != 0 {
		dpt.TopicExcerptBytes = spt.TopicExcerptBytes
	}
	if spt.MemoryPreviewBytes != 0 {
		dpt.MemoryPreviewBytes = spt.MemoryPreviewBytes
	}
	if spt.TimeoutSeconds != 0 {
		dpt.TimeoutSeconds = spt.TimeoutSeconds
	}
	if spt.MaxTokens != 0 {
		dpt.MaxTokens = spt.MaxTokens
	}
	if spt.UserSnapshotBytes != 0 {
		dpt.UserSnapshotBytes = spt.UserSnapshotBytes
	}
	if spt.AssistantSnapshotBytes != 0 {
		dpt.AssistantSnapshotBytes = spt.AssistantSnapshotBytes
	}
	if src.Maintain.ScheduledTimeoutSeconds != 0 {
		dst.Maintain.ScheduledTimeoutSeconds = src.Maintain.ScheduledTimeoutSeconds
	}
	if src.Maintain.ScheduledMaxSteps != 0 {
		dst.Maintain.ScheduledMaxSteps = src.Maintain.ScheduledMaxSteps
	}
	if src.Maintain.IncrementalOverlap != "" {
		dst.Maintain.IncrementalOverlap = src.Maintain.IncrementalOverlap
	}
	if src.Maintain.IncrementalMaxSpan != "" {
		dst.Maintain.IncrementalMaxSpan = src.Maintain.IncrementalMaxSpan
	}
	if src.Log.Level != "" {
		dst.Log.Level = src.Log.Level
	}
	if src.Log.Format != "" {
		dst.Log.Format = src.Log.Format
	}
	if src.SidechainMerge != "" {
		dst.SidechainMerge = src.SidechainMerge
	}
	if src.Usage.DefaultInputPerMtok > 0 {
		dst.Usage.DefaultInputPerMtok = src.Usage.DefaultInputPerMtok
	}
	if src.Usage.DefaultOutputPerMtok > 0 {
		dst.Usage.DefaultOutputPerMtok = src.Usage.DefaultOutputPerMtok
	}
	if src.Schedule.MinSleep != "" {
		dst.Schedule.MinSleep = src.Schedule.MinSleep
	}
	if src.Schedule.IdleSleep != "" {
		dst.Schedule.IdleSleep = src.Schedule.IdleSleep
	}
	if src.SemanticCompact.SummaryMaxBytes != 0 {
		dst.SemanticCompact.SummaryMaxBytes = src.SemanticCompact.SummaryMaxBytes
	}
	if src.Skills.RecentPath != "" {
		dst.Skills.RecentPath = src.Skills.RecentPath
	}
	if src.Clawbridge.Media.Root != "" {
		dst.Clawbridge.Media.Root = src.Clawbridge.Media.Root
	}
	if len(src.Clawbridge.Clients) > 0 {
		dst.Clawbridge.Clients = append([]cbconfig.ClientConfig(nil), src.Clawbridge.Clients...)
	}
	mergeBoolPtr(&dst.Sessions.DisableSQLite, src.Sessions.DisableSQLite)
	if src.Sessions.SQLitePath != "" {
		dst.Sessions.SQLitePath = src.Sessions.SQLitePath
	}
	if src.Sessions.WorkerCount != 0 {
		dst.Sessions.WorkerCount = src.Sessions.WorkerCount
	}
	mergeMCP(&dst.MCP, src.MCP)
}

func mergeBoolPtr(dst **bool, src *bool) {
	if src != nil {
		*dst = src
	}
}

// LoadOptions selects config paths. Cwd and Home should be absolute.
type LoadOptions struct {
	Cwd          string
	Home         string
	ExplicitPath string // --config; highest precedence layer
}

// Load reads and merges YAML layers: user ~/.oneclaw/config.yaml, then project <cwd>/.oneclaw/config.yaml,
// then ExplicitPath when set. Missing optional files are ignored. If ExplicitPath is non-empty and missing,
// returns an error.
func Load(opt LoadOptions) (*Resolved, error) {
	var merged File

	userPath := filepath.Join(opt.Home, UserRelPath)
	if _, err := os.Stat(userPath); err == nil {
		layer, err := readFileLayer(userPath)
		if err != nil {
			return nil, err
		}
		mergeFile(&merged, layer)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	projPath := filepath.Join(opt.Cwd, memory.DotDir, "config.yaml")
	if _, err := os.Stat(projPath); err == nil {
		layer, err := readFileLayer(projPath)
		if err != nil {
			return nil, err
		}
		mergeFile(&merged, layer)
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	explicit := strings.TrimSpace(opt.ExplicitPath)
	if explicit != "" {
		p := explicit
		if !filepath.IsAbs(p) {
			p = filepath.Join(opt.Cwd, p)
		}
		p = filepath.Clean(p)
		if _, err := os.Stat(p); err != nil {
			return nil, err
		}
		layer, err := readFileLayer(p)
		if err != nil {
			return nil, err
		}
		mergeFile(&merged, layer)
	}

	return &Resolved{merged: merged, cwd: opt.Cwd, explicitConfig: explicit}, nil
}
