package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/paths"
)

// Live LLM：`ONECLAW_E2E_LIVE_LLM=1` 且勿使用 `-short` 时，本包内凡调用 executeTurn 的用例均走真实模型（另见 live_mode_helpers_test.go）。
// 凭证：`test/e2e/.env` 或仓库根 `.env`（TestMain 加载），变量 ONECLAW_E2E_* 或别名 base_url / api_key / default_model。
// default_model 可为裸 API 模型名（合并为 openai_compatible/<name>）或已是 profile_id_or_provider/model。
//
// 仅 Live 冒烟：go test ./test/e2e -run TestLiveLLM -count=1 -timeout 20m -v
// 全套集成（耗 API）：ONECLAW_E2E_LIVE_LLM=1 go test ./test/e2e -count=1 -timeout 120m -v
func skipUnlessLiveLLM(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("-short: skipping live LLM")
	}
	if !liveLLMEnabled() {
		t.Skipf("set %s=1 to run live LLM tests (optional; default is mock-only suite)", envLiveLLMFlag)
	}
}

func TestLiveLLM_singleTurn_smoke(t *testing.T) {
	skipUnlessLiveLLM(t)

	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "live-smoke"

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	out, err := executeTurnCtx(t, ctx, root, cfg, sess,
		"Reply in one short English sentence confirming you received this test ping.", false, "")
	if err != nil {
		t.Fatal(err)
	}
	out = strings.TrimSpace(out)
	if len(out) < 8 {
		t.Fatalf("expected non-trivial assistant stdout, got %q", out)
	}
	if strings.Contains(out, stubReply()) {
		t.Fatalf("got stub phrase in stdout — still using mock? output: %q", out)
	}

	sessRoot := paths.SessionRoot(root, sess)
	d := lastRunStartDetail(t, sessRoot, "default")
	if v, ok := d["mock_llm"].(bool); ok && v {
		t.Fatalf("run_start should not mark mock_llm when UseMock=false: %#v", d)
	}
	if n := transcriptLines(t, sessRoot); n < 2 {
		t.Fatalf("want transcript user+assistant after live turn, got %d lines", n)
	}
	assertRunJournalHasPhase(t, sessRoot, "default", "run_complete")
}
