package e2e_test

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/lengzhao/oneclaw/test/e2e/dotenv"
)

// liveLLMConfigPatchYAML merges over bootstrap config: real endpoint + inline api_key from ONECLAW_E2E_* (.env).
func liveLLMConfigPatchYAML(t *testing.T) string {
	t.Helper()
	key := strings.TrimSpace(os.Getenv(dotenv.EnvAPIKey))
	if key == "" {
		key = strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	}
	if key == "" {
		t.Fatal("live LLM: set ONECLAW_E2E_API_KEY / api_key in .env, or OPENAI_API_KEY in environment")
	}
	base := strings.TrimSpace(os.Getenv(dotenv.EnvBaseURL))
	dm := strings.TrimSpace(os.Getenv(dotenv.EnvDefaultModel))

	prof := map[string]any{
		"id":       "default",
		"priority": 0,
		"provider": "openai_compatible",
		"api_key":  key,
	}
	if base != "" {
		prof["base_url"] = base
	}
	if dm != "" {
		prof["default_model"] = dm
	}

	root := map[string]any{"models": []any{prof}}
	if dm != "" {
		root["default_model"] = dm
	}

	b, err := yaml.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
