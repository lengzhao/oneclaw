// Package config loads oneclaw YAML configuration with a single merge rule for dev and production.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"github.com/lengzhao/oneclaw/workspace"
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
		// MaxTokens is max_completion_tokens per model step (main thread / IM sessions). 0 = use Resolved default.
		MaxTokens int64 `yaml:"max_tokens"`
	} `yaml:"agent"`

	Paths struct {
		MemoryBase                   string `yaml:"memory_base"`
		Transcript                   string `yaml:"transcript"`
		WorkingTranscript            string `yaml:"working_transcript"`
		WorkingTranscriptMaxMessages int    `yaml:"working_transcript_max_messages"` // 0=default 30; <0=unlimited
	} `yaml:"paths"`

	Features struct {
		DisableTranscript          *bool `yaml:"disable_transcript"`
		DisableMemory              *bool `yaml:"disable_memory"`
		DisableAutoMemory          *bool `yaml:"disable_auto_memory"`
		DisableContextBudget       *bool `yaml:"disable_context_budget"`
		DisableBehaviorPolicyWrite *bool `yaml:"disable_behavior_policy_write"`
		DisableScheduledTasks      *bool `yaml:"disable_scheduled_tasks"`
		DisableSemanticCompact     *bool `yaml:"disable_semantic_compact"`
		DisableSkills              *bool `yaml:"disable_skills"`
		DisableTasks               *bool `yaml:"disable_tasks"`
		// DisableMultimodalImage: when true, inbound images use read_file hints only (no vision API parts).
		DisableMultimodalImage *bool `yaml:"disable_multimodal_image"`
		// DisableMultimodalAudio: when true, inbound wav/mp3 use read_file hints only (no input_audio parts).
		DisableMultimodalAudio *bool `yaml:"disable_multimodal_audio"`
	} `yaml:"features"`

	Budget struct {
		MaxContextTokens      int `yaml:"max_context_tokens"` // ×2 → byte budget when max_prompt_bytes unset
		MaxPromptBytes        int `yaml:"max_prompt_bytes"`
		MinTranscriptMessages int `yaml:"min_transcript_messages"`
		HistoryMaxBytes       int `yaml:"history_max_bytes"`
		SystemExtraMaxBytes   int `yaml:"system_extra_max_bytes"`
		AgentMdMaxBytes       int `yaml:"agent_md_max_bytes"`
		SkillIndexMaxBytes    int `yaml:"skill_index_max_bytes"`
		InheritedMessages     int `yaml:"inherited_messages"`
	} `yaml:"budget"`

	Log struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
		// File: append logs here (UTF-8) in addition to stderr; relative paths resolve from UserDataRoot (~/.oneclaw).
		File string `yaml:"file"`
	} `yaml:"log"`

	SidechainMerge string `yaml:"sidechain_merge"`

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

	// Sessions: per-chat isolation (IM threads). Transcripts and per-session dirs under sessions/<id>/.
	Sessions struct {
		// IsolateWorkspace: when true, InstructionRoot is <UserDataRoot>/sessions/<id>/ (per-session AGENT/MEMORY/workspace).
		// When false (default), InstructionRoot is UserDataRoot; transcripts remain per-session under sessions/<id>/.
		IsolateWorkspace *bool `yaml:"isolate_workspace"`
		// TurnPolicy: serial (default), insert, preempt — see session.TurnHub / session.ParseTurnPolicy.
		TurnPolicy string `yaml:"turn_policy"`
	} `yaml:"sessions"`

	// Clawbridge is IM 总线配置，形状与 github.com/lengzhao/clawbridge/config.Config 一致（media + clients）。
	// 见 clawbridge 仓库 config.example.yaml。media.root 留空时运行时默认为 <UserDataRoot>/media。
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
	if src.Agent.MaxTokens != 0 {
		dst.Agent.MaxTokens = src.Agent.MaxTokens
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
	mergeBoolPtr(&dst.Features.DisableContextBudget, src.Features.DisableContextBudget)
	mergeBoolPtr(&dst.Features.DisableBehaviorPolicyWrite, src.Features.DisableBehaviorPolicyWrite)
	mergeBoolPtr(&dst.Features.DisableScheduledTasks, src.Features.DisableScheduledTasks)
	mergeBoolPtr(&dst.Features.DisableSemanticCompact, src.Features.DisableSemanticCompact)
	mergeBoolPtr(&dst.Features.DisableSkills, src.Features.DisableSkills)
	mergeBoolPtr(&dst.Features.DisableTasks, src.Features.DisableTasks)
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
	if src.Log.Level != "" {
		dst.Log.Level = src.Log.Level
	}
	if src.Log.Format != "" {
		dst.Log.Format = src.Log.Format
	}
	if src.Log.File != "" {
		dst.Log.File = src.Log.File
	}
	if src.SidechainMerge != "" {
		dst.SidechainMerge = src.SidechainMerge
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
	if src.Sessions.TurnPolicy != "" {
		dst.Sessions.TurnPolicy = src.Sessions.TurnPolicy
	}
	mergeBoolPtr(&dst.Sessions.IsolateWorkspace, src.Sessions.IsolateWorkspace)
	mergeMCP(&dst.MCP, src.MCP)
}

func mergeBoolPtr(dst **bool, src *bool) {
	if src != nil {
		*dst = src
	}
}

// LoadOptions selects config paths. Home must be non-empty (typically os.UserHomeDir()); use absolute paths.
type LoadOptions struct {
	Home         string
	ExplicitPath string // --config; merged after user config; highest precedence
}

// Load reads and merges YAML layers: user ~/.oneclaw/config.yaml, then ExplicitPath when set.
// Relative ExplicitPath is resolved from <home>/.oneclaw/. Missing user config is ignored.
// If ExplicitPath is non-empty and missing, returns an error.
func Load(opt LoadOptions) (*Resolved, error) {
	if strings.TrimSpace(opt.Home) == "" {
		return nil, fmt.Errorf("config.load: empty Home")
	}
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

	explicit := strings.TrimSpace(opt.ExplicitPath)
	if explicit != "" {
		p := explicit
		if !filepath.IsAbs(p) {
			p = filepath.Join(opt.Home, workspace.DotDir, p)
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

	return &Resolved{merged: merged, home: opt.Home, explicitConfig: explicit}, nil
}
