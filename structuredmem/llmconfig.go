package structuredmem

import (
	"strings"

	lzmodel "github.com/lengzhao/memory/model"

	"github.com/lengzhao/oneclaw/config"
)

// LLMConfigFromProfile builds OpenAI-compatible runtime config for lengzhao/memory extraction.
// Returns nil when the profile cannot call HTTP chat completions (mock, missing key/model).
func LLMConfigFromProfile(prof config.ModelProfile) *lzmodel.LLMConfig {
	if strings.EqualFold(strings.TrimSpace(prof.Provider), "mock") {
		return nil
	}
	if strings.TrimSpace(prof.APIKey) == "" {
		return nil
	}
	modelID := strings.TrimSpace(prof.DefaultModel)
	if modelID == "" {
		return nil
	}
	cfg := &lzmodel.LLMConfig{
		APIKey:         prof.APIKey,
		Model:          modelID,
		MaxTokens:      4096,
		Temperature:    0.2,
		TimeoutSeconds: 120,
	}
	if u := strings.TrimSpace(prof.BaseURL); u != "" {
		cfg.BaseURL = &u
	}
	return cfg
}
