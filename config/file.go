// Package config loads merged YAML and runtime snapshots (see docs/reference-architecture.md §2.1).
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	cbconfig "github.com/lengzhao/clawbridge/config"
	"gopkg.in/yaml.v3"
)

// EnvVarNameRE matches portable environment variable names (letters, digits, underscore).
var EnvVarNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// File is the root YAML shape for oneclaw (phase 1 subset).
type File struct {
	UserDataRoot string        `yaml:"user_data_root,omitempty"`
	Catalog      CatalogConfig `yaml:"catalog,omitempty"`
	Sessions     Sessions      `yaml:"sessions,omitempty"`
	// DefaultModel is always "profile_id_or_provider/api_model_id" (first '/' only); see SplitProviderModel.
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

// ModelProfile is one credential binding (account / endpoint / key_files).
// Selector left side matches models[].id first, else models[].provider (failover across same provider).
// IDs must not contain '/' — use SplitProviderModel for agent/default_model strings.
type ModelProfile struct {
	ID       string `yaml:"id,omitempty"`
	Priority int    `yaml:"priority,omitempty"` // lower = higher priority when iterating backups

	Provider string `yaml:"provider,omitempty"` // openai | openai_compatible | claude | gemini | ark | moonshot | qwen | deepseek | openrouter | mock

	BaseURL   string `yaml:"base_url,omitempty"`
	APIKey    string `yaml:"api_key,omitempty"`     // discouraged; prefer api_key_env
	APIKeyEnv string `yaml:"api_key_env,omitempty"` // e.g. OPENAI_API_KEY

	// Auth optional file-backed credentials relative to UserDataRoot (key_files/*).
	Auth ModelAuth `yaml:"auth,omitempty"`

	// DefaultModel on a profile is unused; API model id comes only from profile_id_or_provider/model selectors.
	DefaultModel string `yaml:"default_model,omitempty"`
}

// ModelAuth references on-disk API keys under UserDataRoot (FR-CFG-01 complement; unified-model-auth).
type ModelAuth struct {
	TokenFile string `yaml:"token_file,omitempty"` // relative path, e.g. key_files/dashscope.json
}

// UnmarshalYAML accepts only token_file; legacy auth.provider / grant / kind and any other key are rejected.
func (m *ModelAuth) UnmarshalYAML(n *yaml.Node) error {
	*m = ModelAuth{}
	if n == nil {
		return nil
	}
	switch n.Kind {
	case yaml.ScalarNode:
		if n.Tag == "null" || strings.TrimSpace(n.Value) == "" || n.Value == "~" {
			return nil
		}
		return fmt.Errorf("config: auth must be a mapping or null")
	case yaml.MappingNode:
		for i := 0; i < len(n.Content); i += 2 {
			kn := n.Content[i]
			vn := n.Content[i+1]
			if kn.Kind != yaml.ScalarNode {
				return fmt.Errorf("config: auth mapping keys must be strings")
			}
			key := kn.Value
			switch key {
			case "token_file":
				if err := vn.Decode(&m.TokenFile); err != nil {
					return fmt.Errorf("config: auth.token_file: %w", err)
				}
			case "provider", "grant", "kind":
				return fmt.Errorf("config: auth.%q is not supported; use auth.token_file only", key)
			default:
				return fmt.Errorf("config: unknown auth field %q (allowed: token_file)", key)
			}
		}
		return nil
	default:
		return fmt.Errorf("config: auth must be a mapping or null")
	}
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

// ApplyDefaults ensures at least one profile and per-profile defaults; fills empty root default_model as highest-priority provider/gpt-5.4-nano.
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

	for i := range f.Models {
		if f.Models[i].ID == "" {
			f.Models[i].ID = fmt.Sprintf("model-%d", i)
		}
		applyModelProfileDefaults(&f.Models[i])
	}
	if strings.TrimSpace(f.DefaultModel) == "" && len(f.Models) > 0 {
		idx := sortedFailoverIndices(f.Models)[0]
		f.DefaultModel = strings.TrimSpace(f.Models[idx].Provider) + "/gpt-5.4-nano"
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
	p := normalizedModelProvider(mp.Provider)

	switch p {
	case "qwen":
		if mp.BaseURL == "" {
			mp.BaseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
		}
	case "moonshot":
		if mp.BaseURL == "" {
			mp.BaseURL = "https://api.moonshot.cn/v1"
		}
	case "deepseek":
		if mp.BaseURL == "" {
			mp.BaseURL = "https://api.deepseek.com/"
		}
	case "openrouter":
		if mp.BaseURL == "" {
			mp.BaseURL = "https://openrouter.ai/api/v1"
		}
	case "openai", "openai_compatible":
		if mp.BaseURL == "" {
			mp.BaseURL = "https://api.openai.com/v1"
		}
	case "gemini", "ark", "claude":
		// Optional base_url overrides (Gemini compatible endpoint, Ark region URL, Claude proxy).
	default:
		if mp.BaseURL == "" {
			mp.BaseURL = "https://api.openai.com/v1"
		}
	}

	if strings.TrimSpace(mp.Auth.TokenFile) != "" {
		mp.APIKeyEnv = ""
		return
	}
	if mp.APIKeyEnv == "" && mp.APIKey == "" {
		mp.APIKeyEnv = defaultAPIKeyEnvForProvider(p, mp.BaseURL)
	}
}

func normalizedModelProvider(p string) string {
	return strings.ToLower(strings.TrimSpace(p))
}

func defaultAPIKeyEnvForProvider(providerNormalized, baseURL string) string {
	switch providerNormalized {
	case "claude":
		return "ANTHROPIC_API_KEY"
	case "gemini":
		return "GEMINI_API_KEY"
	case "ark":
		return "ARK_API_KEY"
	case "moonshot":
		return "MOONSHOT_API_KEY"
	case "qwen":
		return "DASHSCOPE_API_KEY"
	case "deepseek":
		return "DEEPSEEK_API_KEY"
	case "openrouter":
		return "OPENROUTER_API_KEY"
	default:
		if strings.Contains(strings.ToLower(baseURL), "moonshot.cn") {
			return "MOONSHOT_API_KEY"
		}
		return "OPENAI_API_KEY"
	}
}

// Validate checks profile ids and root default_model after ApplyDefaults.
func Validate(f *File) error {
	if f == nil {
		return nil
	}
	if len(f.Models) == 0 {
		return fmt.Errorf("config: no model profiles")
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
		if strings.Contains(m.ID, "/") {
			return fmt.Errorf("config: model profile id %q must not contain '/' (use default_model or agent model: profile_id_or_provider/model)", m.ID)
		}
		env := strings.TrimSpace(m.APIKeyEnv)
		if env != "" && !EnvVarNameRE.MatchString(env) {
			return fmt.Errorf("config: profile %q: api_key_env %q is not an environment variable name (you may have pasted the API secret there; use api_key_env: MOONSHOT_API_KEY plus export MOONSHOT_API_KEY=..., or yaml field api_key)", m.ID, m.APIKeyEnv)
		}
		tf := strings.TrimSpace(m.Auth.TokenFile)
		if tf != "" {
			if filepath.IsAbs(tf) || strings.Contains(tf, "..") {
				return fmt.Errorf("config: profile %q: auth.token_file must be relative to user data root and must not contain '..'", m.ID)
			}
		}
		if strings.TrimSpace(m.DefaultModel) != "" {
			return fmt.Errorf("config: profile %q: default_model must not be set (use root default_model: profile_id_or_provider/api_model)", m.ID)
		}
	}
	rootDM := strings.TrimSpace(f.DefaultModel)
	cred, _, ok := SplitProviderModel(rootDM)
	if !ok {
		return fmt.Errorf("config: default_model must be profile_id_or_provider/api_model_id (got %q)", rootDM)
	}
	var credFound bool
	for _, m := range f.Models {
		if m.ID == cred || strings.EqualFold(strings.TrimSpace(m.Provider), cred) {
			credFound = true
			break
		}
	}
	if !credFound {
		return fmt.Errorf("config: default_model left segment %q matches no models[].id nor models[].provider", cred)
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
