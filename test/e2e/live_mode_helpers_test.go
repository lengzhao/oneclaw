package e2e_test

import (
	"os"
	"strings"
	"testing"
)

const envLiveLLMFlag = "ONECLAW_E2E_LIVE_LLM"

// liveLLMEnabled is true when the full integration suite should call real APIs (non-short runs only).
func liveLLMEnabled() bool {
	return strings.TrimSpace(os.Getenv(envLiveLLMFlag)) == "1"
}

// useMockLLM reports false when ONECLAW_E2E_LIVE_LLM=1 and not go test -short.
func useMockLLM(t *testing.T) bool {
	t.Helper()
	if testing.Short() {
		return true
	}
	return !liveLLMEnabled()
}

// cfgPatchForE2E returns merged YAML for real endpoints when live mode is on.
func cfgPatchForE2E(t *testing.T) string {
	t.Helper()
	if testing.Short() || !liveLLMEnabled() {
		return ""
	}
	return liveLLMConfigPatchYAML(t)
}
