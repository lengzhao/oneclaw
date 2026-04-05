// Package config loads oneclaw YAML configuration with a single merge rule for dev and production.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/memory"
	"gopkg.in/yaml.v3"
)

// UserRelPath is the config file under the user's home directory (~/.oneclaw/config.yaml).
const UserRelPath = ".oneclaw/config.yaml"

// File is one YAML layer. Empty strings mean "not set" for merge purposes.
type File struct {
	OpenAI OpenAIFile `yaml:"openai"`
	Model  string     `yaml:"model"`

	Chat struct {
		Transport string `yaml:"transport"`
	} `yaml:"chat"`

	Paths struct {
		MemoryBase  string `yaml:"memory_base"`
		Transcript  string `yaml:"transcript"`
		TurnLogPath string `yaml:"turn_log_path"`
	} `yaml:"paths"`

	Features struct {
		DisableTranscript      *bool `yaml:"disable_transcript"`
		DisableMemory          *bool `yaml:"disable_memory"`
		DisableTurnLog         *bool `yaml:"disable_turn_log"`
		DisableAutoMemory      *bool `yaml:"disable_auto_memory"`
		DisableMemoryExtract   *bool `yaml:"disable_memory_extract"`
		DisableAutoMaintenance *bool `yaml:"disable_auto_maintenance"`
		DisableMemoryAudit     *bool `yaml:"disable_memory_audit"`
		DisableContextBudget   *bool `yaml:"disable_context_budget"`
	} `yaml:"features"`

	Budget struct {
		MaxPromptBytes        int `yaml:"max_prompt_bytes"`
		MinTranscriptMessages int `yaml:"min_transcript_messages"`
		RecallMaxBytes        int `yaml:"recall_max_bytes"`
	} `yaml:"budget"`

	Maintain struct {
		Interval        string `yaml:"interval"`
		Model           string `yaml:"model"`
		ScheduledModel  string `yaml:"scheduled_model"`
		MaxTokens       int64  `yaml:"max_tokens"`
		MinLogBytes     int    `yaml:"min_log_bytes"`
		MaxLogReadBytes int    `yaml:"max_log_bytes"`
	} `yaml:"maintain"`

	Log struct {
		Level  string `yaml:"level"`
		Format string `yaml:"format"`
	} `yaml:"log"`
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
	if src.Chat.Transport != "" {
		dst.Chat.Transport = src.Chat.Transport
	}
	if src.Paths.MemoryBase != "" {
		dst.Paths.MemoryBase = src.Paths.MemoryBase
	}
	if src.Paths.Transcript != "" {
		dst.Paths.Transcript = src.Paths.Transcript
	}
	if src.Paths.TurnLogPath != "" {
		dst.Paths.TurnLogPath = src.Paths.TurnLogPath
	}
	mergeBoolPtr(&dst.Features.DisableTranscript, src.Features.DisableTranscript)
	mergeBoolPtr(&dst.Features.DisableMemory, src.Features.DisableMemory)
	mergeBoolPtr(&dst.Features.DisableTurnLog, src.Features.DisableTurnLog)
	mergeBoolPtr(&dst.Features.DisableAutoMemory, src.Features.DisableAutoMemory)
	mergeBoolPtr(&dst.Features.DisableMemoryExtract, src.Features.DisableMemoryExtract)
	mergeBoolPtr(&dst.Features.DisableAutoMaintenance, src.Features.DisableAutoMaintenance)
	mergeBoolPtr(&dst.Features.DisableMemoryAudit, src.Features.DisableMemoryAudit)
	mergeBoolPtr(&dst.Features.DisableContextBudget, src.Features.DisableContextBudget)
	if src.Budget.MaxPromptBytes != 0 {
		dst.Budget.MaxPromptBytes = src.Budget.MaxPromptBytes
	}
	if src.Budget.MinTranscriptMessages != 0 {
		dst.Budget.MinTranscriptMessages = src.Budget.MinTranscriptMessages
	}
	if src.Budget.RecallMaxBytes != 0 {
		dst.Budget.RecallMaxBytes = src.Budget.RecallMaxBytes
	}
	if src.Maintain.Interval != "" {
		dst.Maintain.Interval = src.Maintain.Interval
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
	if src.Log.Level != "" {
		dst.Log.Level = src.Log.Level
	}
	if src.Log.Format != "" {
		dst.Log.Format = src.Log.Format
	}
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
