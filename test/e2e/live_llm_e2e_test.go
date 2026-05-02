//go:build e2e && live_llm

// 真实 LLM 验收：复制 live_llm.config.example.yaml 为 live_llm.config.yaml 并填写 api_key。
// 默认构建不包含本文件，避免 CI 或无密钥环境失败。
//
//	go test -tags=e2e,live_llm ./test/e2e/... -run TestLiveLLM -count=1 -v
package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
	"github.com/lengzhao/oneclaw/workspace"
	"github.com/openai/openai-go"
)

func testLiveConfigPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "live_llm.config.yaml")
}

func loadLiveResolved(t *testing.T, cfgPath string, home string) *config.Resolved {
	t.Helper()
	absCfg, err := filepath.Abs(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	r, err := config.Load(config.LoadOptions{
		Home:         home,
		ExplicitPath: absCfg,
	})
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	raw, _ := os.ReadFile(absCfg)
	if strings.Contains(string(raw), "YOUR_API_KEY_HERE") {
		t.Skip("replace YOUR_API_KEY_HERE in live_llm.config.yaml with a real key")
	}
	if !r.HasAPIKey() {
		t.Skip("set openai.api_key in test/e2e/live_llm.config.yaml")
	}
	return r
}

func newLiveEngine(t *testing.T, r *config.Resolved, cwd string) *session.Engine {
	t.Helper()
	e := session.NewEngine(cwd, builtin.DefaultRegistry())
	e.Client = openai.NewClient(r.OpenAIOptions()...)
	e.EinoOpenAIAPIKey = r.OpenAIAPIKey()
	e.EinoOpenAIBaseURL = r.OpenAIBaseURL()
	if m := r.ChatModel(); m != "" {
		e.Model = m
	}
	if tr := r.ChatTransport(); tr != "" {
		e.ChatTransport = tr
	}
	return e
}

// TestLiveLLM_ChatRoundTrip 验证密钥与网关：单轮对话，assistant 非空。
func TestLiveLLM_ChatRoundTrip(t *testing.T) {
	if testing.Short() {
		t.Skip("-short")
	}
	cfgPath := testLiveConfigPath(t)
	if _, err := os.Stat(cfgPath); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("missing %s — copy live_llm.config.example.yaml and fill api_key", cfgPath)
		}
		t.Fatal(err)
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("OPENAI_BASE_URL", "")

	r := loadLiveResolved(t, cfgPath, home)
	r.PushRuntime()
	s := rtopts.Current()
	s.DisableMemory = true
	rtopts.Set(&s)
	sessionHome := filepath.Join(r.UserDataRoot(), "sessions", "e2e-live")
	if err := os.MkdirAll(filepath.Join(sessionHome, workspace.DotDir), 0o755); err != nil {
		t.Fatal(err)
	}
	e := newLiveEngine(t, r, sessionHome)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	if err := e.SubmitUser(ctx, bus.InboundMessage{Content: "Say exactly: LIVE_OK"}); err != nil {
		t.Fatal(err)
	}
	out := strings.TrimSpace(loop.LastAssistantDisplay(e.Messages))
	if out == "" {
		t.Fatal("empty assistant reply")
	}
	t.Logf("assistant: %s", out)
}
