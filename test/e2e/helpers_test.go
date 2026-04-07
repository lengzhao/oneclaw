package e2e_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/test/openaistub"
	"github.com/lengzhao/oneclaw/tools"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// stubOpenAIOptions returns client options for openaistub (no process env).
func stubOpenAIOptions(stub *openaistub.Server) []option.RequestOption {
	return []option.RequestOption{
		option.WithAPIKey("sk-test-stub"),
		option.WithBaseURL(stub.BaseURL()),
	}
}

// newStubEngine builds an Engine after e2eEnv*; call e2eEnv* before this.
func newStubEngine(t *testing.T, stub *openaistub.Server, cwd string) *session.Engine {
	t.Helper()
	e := session.NewEngine(cwd, builtin.DefaultRegistry())
	e.MaxTokens = 512
	e.MaxSteps = 16
	e.Client = openai.NewClient(stubOpenAIOptions(stub)...)
	return e
}

// newStubEngineWithRegistry like newStubEngine but custom registry (e.g. empty for unknown-tool test).
func newStubEngineWithRegistry(t *testing.T, stub *openaistub.Server, cwd string, reg *tools.Registry) *session.Engine {
	t.Helper()
	e := session.NewEngine(cwd, reg)
	e.MaxTokens = 512
	e.MaxSteps = 16
	e.Client = openai.NewClient(stubOpenAIOptions(stub)...)
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

// baseStubTransport sets non-stream chat transport via rtopts (API URL/key go through stubOpenAIOptions).
func baseStubTransport(t *testing.T, _ *openaistub.Server) {
	t.Helper()
	t.Cleanup(func() { rtopts.Set(nil) })
	s := rtopts.DefaultSnapshot()
	s.ChatTransport = "non_stream"
	rtopts.Set(&s)
}

// e2eEnvMinimal is for tests that should not load file-based memory (faster, fewer moving parts).
func e2eEnvMinimal(t *testing.T, stub *openaistub.Server) {
	t.Helper()
	baseStubTransport(t, stub)
	s := rtopts.Current()
	s.DisableMemory = true
	s.MemoryBase = ""
	rtopts.Set(&s)
}

// e2eIsolateUserMemory pins paths.memory_base to filepath.Join(home, memory.DotDir) so tests stay under tmp HOME.
// Call after t.Setenv("HOME", home) and after e2eEnvWithMemory. For home == "", clears the override.
func e2eIsolateUserMemory(t *testing.T, home string) {
	t.Helper()
	s := rtopts.Current()
	if strings.TrimSpace(home) == "" {
		s.MemoryBase = ""
	} else {
		s.MemoryBase = filepath.Join(home, memory.DotDir)
	}
	rtopts.Set(&s)
}

// e2eEnvWithMemory keeps stub transport defaults and does not disable memory.
// Use with t.Setenv("HOME", tmpDir) and mkdir .oneclaw under HOME / cwd as needed.
// Disables post-turn model maintenance so stub queues need not account for a second API call (production default is maintenance on).
func e2eEnvWithMemory(t *testing.T, stub *openaistub.Server) {
	t.Helper()
	baseStubTransport(t, stub)
	s := rtopts.Current()
	s.MemoryBase = ""
	s.DisableAutoMaintenance = true
	rtopts.Set(&s)
}
