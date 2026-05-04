package e2e_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/paths"
)

// slog JSON 记录解析：区分 adk_main 三条语义边界（与 wfexec.handleADKMain / debugLogADKMainModelInput 一致）：
// 1) system_prompt — RenderMainAgentPrompt：含 SkillsIndex、MEMORY.md 注入、技能正文等，不含本轮「对话消息」列表。
// 2) chat_messages — 送给 ChatModelAgent 的 []Message：顺序为 [load_transcript 回放] + [可选 MemoryRecall 伪 user 条] + [当前用户句]。
// 3) stub 模型不产生 tool 轨迹；chat_messages 里不应出现 role 为 tool 的段（若出现则说明误把工具链送进本集成路径）。

func captureVerboseJSONLogs(t *testing.T) (buf *bytes.Buffer, restore func()) {
	t.Helper()
	t.Setenv("ONECLAW_VERBOSE_PROMPT", "1")
	var b bytes.Buffer
	h := slog.NewJSONHandler(&b, &slog.HandlerOptions{Level: slog.LevelInfo})
	prev := slog.Default()
	slog.SetDefault(slog.New(h))
	return &b, func() { slog.SetDefault(prev) }
}

func parseSlogJSONRecords(t *testing.T, buf []byte) []map[string]any {
	t.Helper()
	sc := bufio.NewScanner(bytes.NewReader(buf))
	// 单行 JSON；超长 prompt 仍为一行
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	var out []map[string]any
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal(line, &m); err != nil {
			t.Fatalf("log line %d: %v\n%s", lineNum, err, truncate(string(line), 500))
		}
		out = append(out, m)
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	return out
}

// lastADKMainPair 取最近一次 adk_main 的 system_prompt 与其紧邻的下一条 chat_messages（同一轮模型输入）。
func lastADKMainPair(recs []map[string]any) (systemPromptText, chatMessagesText string, ok bool) {
	msgStr := func(r map[string]any) string {
		s, _ := r["msg"].(string)
		return s
	}
	textStr := func(r map[string]any) string {
		s, _ := r["text"].(string)
		return s
	}
	for i := len(recs) - 1; i >= 0; i-- {
		if msgStr(recs[i]) != "wfexec.adk_main.chat_messages" {
			continue
		}
		chatMessagesText = textStr(recs[i])
		for j := i - 1; j >= 0; j-- {
			if msgStr(recs[j]) == "wfexec.adk_main.system_prompt" {
				systemPromptText = textStr(recs[j])
				return systemPromptText, chatMessagesText, true
			}
		}
		return "", chatMessagesText, false
	}
	return "", "", false
}

func splitChatMessageSections(chatBlock string) []string {
	if strings.TrimSpace(chatBlock) == "" {
		return nil
	}
	parts := strings.Split(chatBlock, "\n--- msg ---\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func messageRolePrefix(seg string) string {
	idx := strings.Index(seg, ": ")
	if idx <= 0 {
		return ""
	}
	return strings.TrimSpace(seg[:idx])
}

func TestE2E_ModelInput_transcriptVsMemoryRecallVsSystemPrompt(t *testing.T) {
	root := bootstrapUserData(t)
	cfg := loadRunEnv(t, root, cfgPatchForE2E(t))
	sess := "e2e-layers"
	mock := useMockLLM(t)
	sessRoot := paths.SessionRoot(root, sess)

	mm := memory.MonthUTC(time.Now().UTC())
	memRel := "layer-recall.md"
	memPath := filepath.Join(sessRoot, "memory", mm, memRel)
	if err := os.MkdirAll(filepath.Dir(memPath), 0o755); err != nil {
		t.Fatal(err)
	}
	recallToken := "RECALL_LAYER_TOKEN_9f3c"
	if err := os.WriteFile(memPath, []byte("# x\n"+recallToken+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	firstUser := "e2e-layer-first-turn-unique-a7x"
	secondUser := "e2e-layer-second-turn-unique-b8y"

	if _, err := executeTurn(t, root, cfg, sess, firstUser, mock, ""); err != nil {
		t.Fatal(err)
	}

	logBuf, restore := captureVerboseJSONLogs(t)
	defer restore()

	if _, err := executeTurn(t, root, cfg, sess, secondUser, mock, ""); err != nil {
		t.Fatal(err)
	}

	recs := parseSlogJSONRecords(t, logBuf.Bytes())
	sysText, chatText, ok := lastADKMainPair(recs)
	if !ok || strings.TrimSpace(chatText) == "" {
		t.Fatal("missing wfexec.adk_main.system_prompt / chat_messages pair")
	}

	// 对话历史不进系统提示词（用户原句不应出现在 system_prompt）。
	if strings.Contains(sysText, firstUser) || strings.Contains(sysText, secondUser) {
		t.Fatalf("system_prompt must not contain user turn text; got snippet:\n%s", truncate(sysText, 1200))
	}

	// chat_messages：须含上一轮 user/assistant + Memory recall 段 + 当前用户句。
	if !strings.Contains(chatText, firstUser) {
		t.Fatalf("chat_messages missing transcript replay:\n%s", truncate(chatText, 6000))
	}
	if mock {
		if !strings.Contains(chatText, stubReply()) {
			t.Fatalf("chat_messages missing stub assistant replay:\n%s", truncate(chatText, 6000))
		}
	} else if !strings.Contains(chatText, "assistant:") {
		t.Fatalf("chat_messages missing assistant replay:\n%s", truncate(chatText, 6000))
	}
	if !strings.Contains(chatText, "## Memory recall") || (!strings.Contains(chatText, recallToken) && !strings.Contains(chatText, memRel)) {
		t.Fatalf("chat_messages missing memory recall lane:\n%s", truncate(chatText, 6000))
	}
	if !strings.Contains(chatText, secondUser) {
		t.Fatalf("chat_messages missing current user message:\n%s", truncate(chatText, 6000))
	}

	segs := splitChatMessageSections(chatText)
	if len(segs) < 4 {
		t.Fatalf("want ≥4 message segments (user, assistant, recall, current); got %d: %#v", len(segs), segs)
	}
	if messageRolePrefix(segs[0]) != "user" || !strings.Contains(segs[0], firstUser) {
		t.Fatalf("seg[0] want transcript user: %s", truncate(segs[0], 400))
	}
	if messageRolePrefix(segs[1]) != "assistant" {
		t.Fatalf("seg[1] want transcript assistant: %s", truncate(segs[1], 400))
	}
	if mock {
		if !strings.Contains(segs[1], stubReply()) {
			t.Fatalf("seg[1] stub assistant: %s", truncate(segs[1], 400))
		}
	} else {
		rest := strings.TrimSpace(segs[1])
		const ap = "assistant: "
		if strings.HasPrefix(rest, ap) && len(strings.TrimSpace(strings.TrimPrefix(rest, ap))) < 8 {
			t.Fatalf("seg[1] live assistant too short: %s", truncate(segs[1], 400))
		}
	}
	if messageRolePrefix(segs[2]) != "user" || !strings.Contains(segs[2], "## Memory recall") {
		t.Fatalf("seg[2] want MemoryRecall as user message: %s", truncate(segs[2], 500))
	}
	if messageRolePrefix(segs[len(segs)-1]) != "user" || !strings.Contains(segs[len(segs)-1], secondUser) {
		t.Fatalf("last seg want current user: %s", truncate(segs[len(segs)-1], 400))
	}

	// mock stub：不应出现 tool 角色段。真实模型可能发生 tool 调用，仅在 mock 下断言。
	if mock {
		for i, seg := range segs {
			if messageRolePrefix(seg) == "tool" {
				t.Fatalf("unexpected tool role in model input seg[%d]: %s", i, truncate(seg, 300))
			}
		}
	}
	assertRunJournalHasPhase(t, sessRoot, "default", "run_complete")
}
