//go:build e2e

package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/clawbridge"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/clawbridge/client"
	_ "github.com/lengzhao/clawbridge/drivers"
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
// inboundWithPersistedAttachments normalizes and persists session attachments, then builds bus.InboundMessage MediaPaths.
func inboundWithPersistedAttachments(t *testing.T, cwd, content, channel, peerID string, atts []session.Attachment) bus.InboundMessage {
	t.Helper()
	atts = append([]session.Attachment(nil), atts...)
	atts = session.NormalizeAttachments(atts)
	if err := session.PersistInlineAttachmentFiles(cwd, &atts); err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, a := range atts {
		if p := strings.TrimSpace(a.Path); p != "" {
			paths = append(paths, p)
		}
	}
	return bus.InboundMessage{
		ClientID:   channel,
		Content:    strings.TrimSpace(content),
		Peer:       bus.Peer{ID: peerID},
		MediaPaths: paths,
	}
}

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

// e2eWaitMinChatRequests polls until the stub has recorded at least want POST /v1/chat/completions bodies.
func e2eWaitMinChatRequests(t *testing.T, stub *openaistub.Server, want int, deadline time.Duration) {
	t.Helper()
	deadlineAt := time.Now().Add(deadline)
	for len(stub.ChatRequestBodies()) < want {
		if time.Now().After(deadlineAt) {
			t.Fatalf("timed out after %v waiting for %d chat requests (have %d)", deadline, want, len(stub.ChatRequestBodies()))
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// e2eWaitForFile polls until path exists and is readable (post-turn maintain runs in a goroutine after the last stub chat body is recorded).
func e2eWaitForFile(t *testing.T, path string, deadline time.Duration) []byte {
	t.Helper()
	deadlineAt := time.Now().Add(deadline)
	for time.Now().Before(deadlineAt) {
		raw, err := os.ReadFile(path)
		if err == nil {
			return raw
		}
		time.Sleep(10 * time.Millisecond)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("timed out after %v waiting for %s: %v", deadline, path, err)
	}
	return raw
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

// e2eStartNoopBridge installs the process-default clawbridge with noop drivers for clientIDs.
// Optional captureMsg is invoked from OutboundSendNotify after each outbound send (for assertions).
// Call cleanup in defer; do not use t.Parallel in tests that rely on this helper.
// e2eWaitOutboundDispatch waits until PublishOutbound has been delivered through the
// bridge outbound loop and the noop driver's Send completed (OutboundSendNotify).
func e2eWaitOutboundDispatch(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if ok() {
			return
		}
		time.Sleep(2 * time.Millisecond)
	}
	t.Fatal("timeout waiting for outbound dispatch / notify")
}

func e2eStartNoopBridge(t *testing.T, clientIDs []string, captureMsg func(*bus.OutboundMessage)) (cleanup func()) {
	t.Helper()
	cfgs := make([]clawbridge.ClientConfig, len(clientIDs))
	for i, id := range clientIDs {
		cfgs[i] = clawbridge.ClientConfig{ID: id, Driver: "noop", Enabled: true}
	}
	var opts []clawbridge.Option
	if captureMsg != nil {
		opts = append(opts, clawbridge.WithOutboundSendNotify(func(ctx context.Context, info client.OutboundSendNotifyInfo) {
			if info.Message != nil {
				captureMsg(info.Message)
			}
		}))
	}
	b, err := clawbridge.New(clawbridge.Config{Clients: cfgs}, opts...)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	clawbridge.SetDefault(b)
	return func() {
		cancel()
		stopCtx, c := context.WithTimeout(context.Background(), 2*time.Second)
		defer c()
		_ = b.Stop(stopCtx)
		clawbridge.SetDefault(nil)
	}
}
