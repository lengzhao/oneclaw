package e2e_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/paths"
)

func TestLiveLLM_twoTurn_codewordEcho(t *testing.T) {
	skipUnlessLiveLLM(t)

	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "live-2turn"
	codeword := "LIVE_E2E_TURN_TOKEN_q7Y"

	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel1()
	out1, err := executeTurnCtx(t, ctx1, root, cfg, sess,
		"Remember this exact codeword for our next message: "+codeword+". Reply with exactly the word OK.", false, "")
	if err != nil {
		t.Fatal(err)
	}
	out1 = strings.TrimSpace(out1)
	if len(out1) < 2 {
		t.Fatalf("first turn stdout too short: %q", out1)
	}
	if strings.Contains(out1, stubReply()) {
		t.Fatalf("first turn still stub output: %q", out1)
	}
	sessRoot := paths.SessionRoot(root, sess)
	if n := transcriptLines(t, sessRoot); n < 2 {
		t.Fatalf("after turn 1 want ≥2 transcript lines, got %d", n)
	}
	assertRunJournalHasPhase(t, sessRoot, "default", "run_complete")

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel2()
	out2, err := executeTurnCtx(t, ctx2, root, cfg, sess,
		"What exact codeword did I ask you to remember in my previous user message? Reply with only that codeword token, no punctuation or extra words.", false, "")
	if err != nil {
		t.Fatal(err)
	}
	out2 = strings.TrimSpace(out2)
	if !strings.Contains(out2, codeword) {
		t.Fatalf("expected assistant to echo codeword %q; got %q", codeword, truncate(out2, 800))
	}
	if n := transcriptLines(t, sessRoot); n < 4 {
		t.Fatalf("want ≥4 transcript lines (2 turns × user+assistant), got %d", n)
	}
	assertRunJournalHasPhase(t, sessRoot, "default", "run_complete")
}

func TestLiveLLM_runStart_hasModelMetadata(t *testing.T) {
	skipUnlessLiveLLM(t)

	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "live-meta"

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	if _, err := executeTurnCtx(t, ctx, root, cfg, sess, "Reply with one word: pong.", false, ""); err != nil {
		t.Fatal(err)
	}

	d := lastRunStartDetail(t, paths.SessionRoot(root, sess), "default")
	modelStr, ok := d["model"].(string)
	if !ok || strings.TrimSpace(modelStr) == "" {
		t.Fatalf("run_start.detail.model missing or empty: %#v", d["model"])
	}
	if strings.EqualFold(strings.TrimSpace(modelStr), "stub") {
		t.Fatalf("live run_start.model should not be stub catalog id: %q", modelStr)
	}
	if prof, ok := d["profile"].(string); !ok || strings.TrimSpace(prof) == "" {
		t.Fatalf("run_start.detail.profile missing or empty: %#v", d["profile"])
	}
	if wf, ok := d["workflow"].(string); !ok || wf != "default.turn" {
		t.Fatalf("run_start.detail.workflow want default.turn: %#v", d["workflow"])
	}
	if v, ok := d["mock_llm"].(bool); ok && v {
		t.Fatalf("run_start should not be mock: %#v", d)
	}
	assertRunJournalHasPhase(t, paths.SessionRoot(root, sess), "default", "run_complete")
}

func TestLiveLLM_memoryRecallInVerboseChatLog(t *testing.T) {
	skipUnlessLiveLLM(t)

	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "live-memrec"

	mm := memory.MonthUTC(time.Now().UTC())
	memTok := "RECALL_LIVE_LAYER_m4p"
	sessSeg := paths.SanitizeSessionPathSegment(sess)
	instrRoot := paths.InstructionRoot(root, sessSeg, cfg.IsolateInstructionOrDefault())
	memPath := filepath.Join(instrRoot, "memory", mm, "live-layer.md")
	if err := os.MkdirAll(filepath.Dir(memPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(memPath, []byte("# live e2e\n"+memTok+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	logBuf, restore := captureVerboseJSONLogs(t)
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	if _, err := executeTurnCtx(t, ctx, root, cfg, sess, "Acknowledge briefly in one short sentence.", false, ""); err != nil {
		t.Fatal(err)
	}

	recs := parseSlogJSONRecords(t, logBuf.Bytes())
	sysText, chatText, ok := lastADKMainPair(recs)
	if !ok {
		t.Fatal("missing adk_main log pair")
	}
	if !strings.Contains(chatText, "## Memory recall") || (!strings.Contains(chatText, memTok) && !strings.Contains(chatText, "live-layer.md")) {
		t.Fatalf("chat_messages missing memory recall lane:\n%s", truncate(chatText, 8000))
	}
	if strings.Contains(sysText, "## Memory recall") {
		t.Fatalf("MemoryRecall must not appear in system_prompt; snippet:\n%s", truncate(sysText, 2500))
	}
	if strings.Contains(sysText, memTok) {
		t.Fatalf("memory token leaked into system_prompt: snippet:\n%s", truncate(sysText, 2500))
	}
	segs := splitChatMessageSections(chatText)
	var recallSeg string
	for _, seg := range segs {
		if strings.Contains(seg, "## Memory recall") {
			recallSeg = seg
			break
		}
	}
	if recallSeg == "" || messageRolePrefix(recallSeg) != "user" {
		t.Fatalf("want Memory recall as a user-role segment; segs=%d recallSeg=%q", len(segs), truncate(recallSeg, 400))
	}
	if messageRolePrefix(segs[len(segs)-1]) != "user" {
		t.Fatalf("last chat segment should be current user turn: %s", truncate(segs[len(segs)-1], 400))
	}
	assertRunJournalHasPhase(t, paths.SessionRoot(root, sessSeg), "default", "run_complete")
}
