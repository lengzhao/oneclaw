package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/lengzhao/oneclaw/test/openaistub"
)

// newStubEngine builds an Engine after stub env is applied; call e2eEnv* before this.
func newStubEngine(t *testing.T, cwd string) *session.Engine {
	t.Helper()
	e := session.NewEngine(cwd, builtin.DefaultRegistry())
	e.MaxTokens = 512
	e.MaxSteps = 16
	return e
}

// newStubEngineWithRegistry like newStubEngine but custom registry (e.g. empty for unknown-tool test).
func newStubEngineWithRegistry(t *testing.T, cwd string, reg *tools.Registry) *session.Engine {
	t.Helper()
	e := session.NewEngine(cwd, reg)
	e.MaxTokens = 512
	e.MaxSteps = 16
	return e
}

// concatUserText joins all user-role string contents (in order) for assertions on injected context.
func concatUserText(msgs []openai.ChatCompletionMessageParamUnion) string {
	var sb strings.Builder
	for _, m := range msgs {
		if m.OfUser == nil {
			continue
		}
		c := m.OfUser.Content
		if c.OfString.Valid() {
			sb.WriteString(c.OfString.Value)
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}

// baseStubTransport sets OPENAI_BASE_URL, OPENAI_API_KEY, and non-stream transport.
// Call before openai.NewClient(). Does not touch memory-related env.
func baseStubTransport(t *testing.T, stub *openaistub.Server) {
	t.Helper()
	t.Setenv("OPENAI_BASE_URL", stub.BaseURL())
	t.Setenv("OPENAI_API_KEY", "sk-test-stub")
	t.Setenv("ONCLAW_CHAT_TRANSPORT", "non_stream")
}

// e2eEnvMinimal is for tests that should not load file-based memory (faster, fewer moving parts).
func e2eEnvMinimal(t *testing.T, stub *openaistub.Server) {
	t.Helper()
	baseStubTransport(t, stub)
	t.Setenv("ONCLAW_DISABLE_MEMORY", "1")
	t.Setenv("ONCLAW_MEMORY_BASE", "")
}

// e2eIsolateUserMemory pins ONCLAW_MEMORY_BASE to filepath.Join(home, memory.DotDir) so a developer
// ONCLAW_MEMORY_BASE (e.g. from .env) cannot write under the repo. Call after t.Setenv("HOME", home)
// and after e2eEnvWithMemory. For home == "", clears the override.
func e2eIsolateUserMemory(t *testing.T, home string) {
	t.Helper()
	if strings.TrimSpace(home) == "" {
		t.Setenv("ONCLAW_MEMORY_BASE", "")
		return
	}
	t.Setenv("ONCLAW_MEMORY_BASE", filepath.Join(home, memory.DotDir))
}

// e2eEnvWithMemory keeps stub transport defaults and does not disable memory.
// Use with t.Setenv("HOME", tmpDir) and mkdir .oneclaw under HOME / cwd as needed.
// Disables post-turn model maintenance so stub queues need not account for a second API call (production default is maintenance on).
func e2eEnvWithMemory(t *testing.T, stub *openaistub.Server) {
	t.Helper()
	t.Setenv("ONCLAW_MEMORY_BASE", "")
	baseStubTransport(t, stub)
	t.Setenv("ONCLAW_DISABLE_AUTO_MAINTENANCE", "1")
}
