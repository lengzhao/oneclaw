//go:build e2e && live_llm

// 真实 LLM 验收：复制 live_llm.config.example.yaml 为 live_llm.config.yaml 并填写 api_key。
// 默认构建不包含本文件，避免 CI 或无密钥环境失败。
//
//	go test -tags=e2e,live_llm ./test/e2e/... -run TestLiveLLM -count=1 -v
package e2e_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/tools/builtin"
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
	if bytes.Contains(raw, []byte("YOUR_API_KEY_HERE")) {
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
	if err := os.MkdirAll(filepath.Join(sessionHome, memory.DotDir), 0o755); err != nil {
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

// TestLiveLLM_DailyLogExtract 验证自学/进化链路之一：PostTurn 写入当日 daily log（需开启 memory 与 extract）。
func TestLiveLLM_DailyLogExtract(t *testing.T) {
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
	s2 := rtopts.Current()
	s2.MemoryBase = filepath.Join(home, memory.DotDir)
	s2.DisableMemory = false
	s2.DisableMemoryExtract = false
	s2.DisableAutoMemory = false
	rtopts.Set(&s2)

	sessionHome := filepath.Join(r.UserDataRoot(), "sessions", "e2e-live-mem")
	if err := os.MkdirAll(filepath.Join(sessionHome, memory.DotDir), 0o755); err != nil {
		t.Fatal(err)
	}
	e := newLiveEngine(t, r, sessionHome)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	token := "E2E_LIVE_MEM_TOKEN_9911"
	msg := "For the project log: remember token " + token + " for testing. Reply with one short acknowledgment sentence only."
	if err := e.SubmitUser(ctx, bus.InboundMessage{Content: msg}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(loop.LastAssistantDisplay(e.Messages)) == "" {
		t.Fatal("empty assistant reply")
	}

	layout := memory.DefaultLayout(sessionHome, home)
	today := time.Now().Format("2006-01-02")
	logPath := memory.DailyLogPath(layout.Auto, today)
	b, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("daily log not written at %s: %v", logPath, err)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		t.Fatalf("daily log empty: %s", logPath)
	}
	if !bytes.Contains(b, []byte(token)) {
		t.Logf("log path: %s", logPath)
		t.Logf("log content:\n%s", b)
		t.Fatalf("daily log does not contain token %q (extract line may diverge; check model/output)", token)
	}
}
