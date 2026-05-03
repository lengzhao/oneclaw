// Package config loads merged YAML and runtime snapshots (see docs/reference-architecture.md §2.1).
package config

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"gopkg.in/yaml.v3"
)

// EnvVarNameRE matches portable environment variable names (letters, digits, underscore).
var EnvVarNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// File is the root YAML shape for oneclaw (phase 1 subset).
type File struct {
	UserDataRoot string   `yaml:"user_data_root,omitempty"`
	Sessions     Sessions `yaml:"sessions,omitempty"`
	// DefaultModel is optional root-level model id applied to any profile whose default_model is empty before built-in fallback.
	DefaultModel string         `yaml:"default_model,omitempty"`
	Models       []ModelProfile `yaml:"models,omitempty"`
	Runtime      RuntimeOptions `yaml:"runtime,omitempty"`
	// Tools toggles builtins (phase 4b); catalog tools: allowlist still applies per agent.
	Tools map[string]ToolSwitch `yaml:"tools,omitempty"`
	// Clawbridge is the multi-channel bus (webchat, IM, …); used by `oneclaw serve` (github.com/lengzhao/clawbridge).
	Clawbridge cbconfig.Config `yaml:"clawbridge,omitempty"`
}

// Sessions mirrors appendix-data-layout isolation switches.
type Sessions struct {
	// IsolateInstructionRoot defaults to true when unset (see ApplyDefaults).
	IsolateInstructionRoot *bool `yaml:"isolate_instruction_root,omitempty"`
}

// ModelProfile is one chat backend account / provider binding.
// Agents or runners pick a profile by id; failover callers use OrderedModelProfiles (Priority ascending).
type ModelProfile struct {
	ID       string `yaml:"id,omitempty"`
	Priority int    `yaml:"priority,omitempty"` // lower = higher priority when iterating backups

	Provider string `yaml:"provider,omitempty"` // openai_compatible | mock

	BaseURL   string `yaml:"base_url,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`     // discouraged; prefer api_key_env
	APIKeyEnv string `yaml:"api_key_env,omitempty"` // e.g. OPENAI_API_KEY

	DefaultModel string `yaml:"default_model,omitempty"`
}

// RuntimeOptions holds cross-cutting execution limits (YAML key "runtime").
type RuntimeOptions struct {
	MaxAgentIterations int `yaml:"max_agent_iterations,omitempty"`
	// MaxDelegationDepth caps nested run_agent depth (each increment runs one sub-agent). Zero applies default in ApplyDefaults.
	MaxDelegationDepth int `yaml:"max_delegation_depth,omitempty"`
}

// IsolateInstructionOrDefault returns sessions.isolate_instruction_root with recommended default true.
func (f *File) IsolateInstructionOrDefault() bool {
	if f == nil || f.Sessions.IsolateInstructionRoot == nil {
		return true
	}
	return *f.Sessions.IsolateInstructionRoot
}

// ApplyDefaults ensures at least one profile, applies root DefaultModel where needed, then per-profile defaults.
func ApplyDefaults(f *File) {
	if f == nil {
		return
	}
	if f.Sessions.IsolateInstructionRoot == nil {
		t := true
		f.Sessions.IsolateInstructionRoot = &t
	}

	if len(f.Models) == 0 {
		f.Models = []ModelProfile{{ID: "default"}}
	}

	globalDM := strings.TrimSpace(f.DefaultModel)
	for i := range f.Models {
		if f.Models[i].ID == "" {
			f.Models[i].ID = fmt.Sprintf("model-%d", i)
		}
		if f.Models[i].DefaultModel == "" && globalDM != "" {
			f.Models[i].DefaultModel = globalDM
		}
		applyModelProfileDefaults(&f.Models[i])
	}

	if f.Runtime.MaxAgentIterations == 0 {
		f.Runtime.MaxAgentIterations = 100
	}
	if f.Runtime.MaxDelegationDepth == 0 {
		f.Runtime.MaxDelegationDepth = 3
	}
}

func applyModelProfileDefaults(mp *ModelProfile) {
	if mp.Provider == "" {
		mp.Provider = "openai_compatible"
	}
	if mp.BaseURL == "" {
		mp.BaseURL = "https://api.openai.com/v1"
	}
	if mp.DefaultModel == "" {
		mp.DefaultModel = "gpt-5.4-nano"
	}
	if mp.APIKeyEnv == "" && mp.APIKey == "" {
		if strings.Contains(strings.ToLower(mp.BaseURL), "moonshot.cn") {
			mp.APIKeyEnv = "MOONSHOT_API_KEY"
		} else {
			mp.APIKeyEnv = "OPENAI_API_KEY"
		}
	}
}

// Validate checks profile ids after ApplyDefaults.
func Validate(f *File) error {
	if f == nil {
		return nil
	}
	seen := make(map[string]bool, len(f.Models))
	for _, m := range f.Models {
		if m.ID == "" {
			return fmt.Errorf("config: model profile has empty id")
		}
		if seen[m.ID] {
			return fmt.Errorf("config: duplicate model profile id %q", m.ID)
		}
		seen[m.ID] = true
		env := strings.TrimSpace(m.APIKeyEnv)
		if env != "" && !EnvVarNameRE.MatchString(env) {
			return fmt.Errorf("config: profile %q: api_key_env %q is not an environment variable name (you may have pasted the API secret there; use api_key_env: MOONSHOT_API_KEY plus export MOONSHOT_API_KEY=..., or yaml field api_key)", m.ID, m.APIKeyEnv)
		}
	}
	return nil
}

// Load reads a single YAML file into File (no merge).
func Load(path string) (*File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(raw, &f); err != nil {
		return nil, err
	}
	ApplyDefaults(&f)
	if err := Validate(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

// LoadMerged deep-merges YAML maps from paths in order (later wins). Empty paths are skipped.
func LoadMerged(paths []string) (*File, error) {
	merged := map[string]any{}
	for _, p := range paths {
		if p == "" {
			continue
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var chunk map[string]any
		if err := yaml.Unmarshal(raw, &chunk); err != nil {
			return nil, err
		}
		merged = mergeMaps(merged, chunk)
	}
	out, err := yaml.Marshal(merged)
	if err != nil {
		return nil, err
	}
	var f File
	if err := yaml.Unmarshal(out, &f); err != nil {
		return nil, err
	}
	ApplyDefaults(&f)
	if err := Validate(&f); err != nil {
		return nil, err
	}
	return &f, nil
}
