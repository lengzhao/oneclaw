package wfexec

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/lengzhao/oneclaw/config"
	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/structuredmem"
	"github.com/lengzhao/oneclaw/workflow"
)

const (
	maxInlineImageCount = 4
	maxInlineImageBytes = 5 << 20
)

// RegisterBuiltins registers handlers for each entry in workflow.BuiltinUses.
func RegisterBuiltins(r *Registry) error {
	if r == nil {
		return fmt.Errorf("wfexec: nil registry")
	}
	byUse := map[string]Handler{
		"on_receive":                handleOnReceive,
		"llm":                       handleLLM,
		"on_respond":                handleOnRespond,
		"agent_task":                handleAgentTask,
		"structured_memory_extract": handleStructuredMemoryExtract,
		"retrieve_context":          handlePassthroughTextNode,
		"command":                   handleWorkflowCommand,
		"tool_call":                 handleWorkflowToolCall,
		"noop":                      handleNoop,
	}
	for _, use := range workflow.BuiltinUses {
		h, ok := byUse[use]
		if !ok {
			return fmt.Errorf("wfexec: missing builtin handler for %q (sync with workflow.BuiltinUses)", use)
		}
		if err := r.Register(use, h); err != nil {
			return err
		}
	}
	return nil
}

func handleOnReceive(_ context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	rtx := env.Runtime
	if strings.TrimSpace(in.Text) != "" {
		rtx.UserPrompt = strings.TrimSpace(in.Text)
	}
	if strings.TrimSpace(rtx.EffectiveUserPrompt()) == "" {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: on_receive: empty user prompt")
	}
	return workflow.WorkflowNodeResult{Text: strings.TrimSpace(rtx.EffectiveUserPrompt())}, nil
}

func handlePassthroughTextNode(_ context.Context, in NodeInput, _ NodeEnv) (workflow.WorkflowNodeResult, error) {
	return workflow.WorkflowNodeResult{Text: strings.TrimSpace(in.Text)}, nil
}

func composeMemoryRecallPrompt(structuredHits, treeListing string) string {
	structuredHits = strings.TrimSpace(structuredHits)
	treeListing = strings.TrimSpace(treeListing)
	if structuredHits == "" && treeListing == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Memory recall\n\n")
	if structuredHits != "" {
		b.WriteString("### Structured memory (lengzhao/memory)\n\n")
		b.WriteString(structuredHits)
		b.WriteString("\n\n")
	}
	if treeListing != "" {
		b.WriteString("### Memory files (paths)\n\n")
		b.WriteString(treeListing)
		b.WriteString("\n\n")
	}
	b.WriteString("_Primary retrieval is SQLite FTS above when present. Use `read_file` with paths under `memory/` when you need full markdown._")
	return strings.TrimRight(b.String(), "\n")
}

func handleLoadMemorySnapshot(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	if contextDisabled(rtx, "memory_recall") {
		slog.InfoContext(rtx.GoCtx, "wfexec.memory_recall.disabled",
			"agent_type", runtimeAgentType(rtx),
			"correlation_id", strings.TrimSpace(rtx.CorrelationID),
		)
		return nil
	}
	budget := preturn.CoalesceBudget(preturn.DefaultBudget())
	q := strings.TrimSpace(rtx.EffectiveUserPrompt())
	structured := ""
	if q != "" {
		structured = strings.TrimSpace(structuredmem.RecallHitsMarkdown(rtx.GoCtx, rtx.EffectiveInstructionRoot(), q, rtx.EffectiveSessionSegment(), runtimeAgentType(rtx), budget.MemoryMaxRunes))
	}
	tree := strings.TrimSpace(preturn.MemoryTreeListingMarkdown(rtx.EffectiveInstructionRoot(), budget))
	block := composeMemoryRecallPrompt(structured, tree)
	rtx.SetPromptTemplateEntry("MemoryRecall", block)
	if strings.TrimSpace(block) == "" {
		slog.InfoContext(rtx.GoCtx, "wfexec.memory_recall.empty",
			"agent_type", runtimeAgentType(rtx),
			"correlation_id", strings.TrimSpace(rtx.CorrelationID),
			"instruction_root", strings.TrimSpace(rtx.EffectiveInstructionRoot()),
		)
		return nil
	}
	slog.InfoContext(rtx.GoCtx, "wfexec.memory_recall.loaded",
		"agent_type", runtimeAgentType(rtx),
		"correlation_id", strings.TrimSpace(rtx.CorrelationID),
		"instruction_root", strings.TrimSpace(rtx.EffectiveInstructionRoot()),
		"chars", len(block),
	)
	return nil
}

func handleListSkills(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	if contextDisabled(rtx, "skills") {
		return nil
	}
	skillsRoot := filepath.Join(paths.CatalogRoot(strings.TrimSpace(rtx.UserDataRoot)), "skills")
	var catalogSkills []string
	if rtx.Agent != nil {
		catalogSkills = rtx.Agent.ReferencedSkillIDs
	}
	s, err := preturn.SkillsDigestMarkdown(skillsRoot, preturn.CoalesceBudget(preturn.DefaultBudget()), catalogSkills)
	if err != nil {
		return fmt.Errorf("wfexec: list_skills: %w", err)
	}
	if strings.TrimSpace(s) == "" {
		s = "(no skills under user-data skills/ yet)"
	}
	rtx.SetPromptTemplateEntry("SkillsIndex", s)
	return nil
}

func handleListTasks(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	if contextDisabled(rtx, "tasks") {
		return nil
	}
	p := filepath.Join(rtx.EffectiveInstructionRoot(), "todo.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			rtx.SetPromptTemplateEntry("Tasks", "(no todo.json — use the `todo` tool with action=list after adding tasks)")
			return nil
		}
		return fmt.Errorf("wfexec: list_tasks: %w", err)
	}
	rtx.SetPromptTemplateEntry("Tasks", strings.TrimSpace(string(b)))
	return nil
}

func handleLoadTranscript(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	if contextDisabled(rtx, "transcript") {
		return nil
	}
	turns, err := session.LoadTranscriptTurns(rtx.EffectiveSessionRoot(), runtimeAgentType(rtx))
	if err != nil {
		return fmt.Errorf("wfexec: load_transcript: %w", err)
	}
	turns = session.TrimTranscriptTail(turns, session.DefaultTranscriptTurnLimit)
	rtx.SetTranscriptReplayTurns(turns)
	return nil
}

func handleNoop(context.Context, NodeInput, NodeEnv) (workflow.WorkflowNodeResult, error) {
	return workflow.WorkflowNodeResult{}, nil
}

func handleLLM(_ context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	rtx := env.Runtime
	if strings.TrimSpace(in.Text) != "" {
		rtx.UserPrompt = strings.TrimSpace(in.Text)
	}
	at := strings.TrimSpace(env.Node.AgentType)
	if at == "" {
		at = workflow.AgentTypeParam(env.Node.Params)
	}
	if at != "" && (rtx.Agent == nil || at != strings.TrimSpace(rtx.Agent.AgentType)) {
		reply, err := executeAgentTask(rtx, at, strings.TrimSpace(in.Text))
		if err != nil {
			return workflow.WorkflowNodeResult{}, err
		}
		return workflow.WorkflowNodeResult{Text: reply}, nil
	}
	if err := runMainLLM(rtx); err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	return workflow.WorkflowNodeResult{Text: strings.TrimSpace(rtx.Assistant)}, nil
}

func runMainLLM(rtx *engine.RuntimeContext) error {
	if rtx.ChatAgent == nil {
		return fmt.Errorf("wfexec: adk_main: ChatAgent not configured")
	}
	if err := prepareAgentContext(rtx); err != nil {
		return err
	}
	instr, err := RenderMainAgentPrompt(rtx)
	if err != nil {
		return err
	}
	if err := rebuildChatAgentForInstruction(rtx, instr); err != nil {
		return fmt.Errorf("wfexec: adk_main: %w", err)
	}
	basePrompt := strings.TrimSpace(rtx.EffectiveUserPrompt())
	if basePrompt == "" {
		return fmt.Errorf("wfexec: adk_main: empty user prompt")
	}
	cur := strings.TrimSpace(workflow.ComposeUserPrompt(rtx.WorkflowMeta, basePrompt))
	if cur == "" {
		return fmt.Errorf("wfexec: adk_main: empty user prompt")
	}
	userTranscript := cur
	if recordSummaryTranscript(rtx) {
		userTranscript = fmt.Sprintf("[%s] async task · full prompt in Run Journal", runtimeAgentType(rtx))
	}
	if !rtx.UserTurnAppended {
		if err := session.AppendTranscriptTurn(rtx.EffectiveSessionRoot(), runtimeAgentType(rtx), session.TranscriptTurn{
			Ts: time.Now().UTC(), Role: "user", Content: userTranscript,
		}); err != nil {
			return fmt.Errorf("wfexec: adk_main: append user transcript: %w", err)
		}
		rtx.UserTurnAppended = true
		appendRunJournalEntry(rtx, "user_message", map[string]any{"content": cur})
	}
	// Model input: [optional transcript history] + [optional memory-recall user message] + [current user message].
	// System instruction is ChatAgent.Instruction only (RenderMainAgentPrompt, no MemoryRecall in template).
	msgs, err := adkMessagesForMain(rtx)
	if err != nil {
		return fmt.Errorf("wfexec: adk_main: %w", err)
	}
	debugLogADKMainModelInput(rtx.GoCtx, rtx, instr, msgs)
	input := &adk.AgentInput{
		Messages: msgs,
	}
	iter := rtx.ChatAgent.Run(rtx.GoCtx, input)
	var chunks []string
	seenToolCalls := make(map[string]bool)
	seenToolResults := make(map[string]bool)
	for {
		ev, ok := iter.Next()
		if !ok {
			break
		}
		if ev.Err != nil {
			return ev.Err
		}
		if ev.Output != nil && ev.Output.MessageOutput != nil && ev.Output.MessageOutput.Message != nil {
			mv := ev.Output.MessageOutput
			msg := mv.Message
			role := mv.Role
			if msg.Role != "" {
				role = msg.Role
			}
			logToolActivity(rtx.GoCtx, rtx, msg, seenToolCalls, seenToolResults)
			if role == schema.Tool {
				continue
			}
			c := strings.TrimSpace(msg.Content)
			if c == "" {
				continue
			}
			chunks = append(chunks, c)
			if rtx.Stdout != nil && rtx.OnAssistantChunk == nil {
				fmt.Fprintln(rtx.Stdout, c)
			}
			if rtx.OnAssistantChunk != nil {
				rtx.OnAssistantChunk(c)
			}
		}
	}
	// Join assistant MessageOutputs (intermediate model text included). Tool result outputs (Role tool) are omitted.
	if len(chunks) == 0 {
		rtx.SetAssistant("")
	} else {
		rtx.SetAssistant(strings.TrimSpace(strings.Join(chunks, "\n")))
	}
	rtx.EmitNodeOutput(map[string]any{
		"use":               "llm",
		"assistant_text":    rtx.Assistant,
		"user_prompt":       rtx.UserPrompt,
		"user_prompt_model": cur,
	})
	return nil
}

func prepareAgentContext(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	if _, ok := rtx.PromptTemplateRaw("MemoryRecall"); !ok && !contextDisabled(rtx, "memory_recall") {
		if err := handleLoadMemorySnapshot(rtx); err != nil {
			return err
		}
	}
	if _, ok := rtx.PromptTemplateRaw("SkillsIndex"); !ok && !contextDisabled(rtx, "skills") {
		if err := handleListSkills(rtx); err != nil {
			return err
		}
	}
	if _, ok := rtx.PromptTemplateRaw("Tasks"); !ok && !contextDisabled(rtx, "tasks") {
		if err := handleListTasks(rtx); err != nil {
			return err
		}
	}
	if rtx.TranscriptReplayTurns == nil && !contextDisabled(rtx, "transcript") {
		if err := handleLoadTranscript(rtx); err != nil {
			return err
		}
	}
	return nil
}

func contextDisabled(rtx *engine.RuntimeContext, block string) bool {
	return rtx != nil && rtx.Agent != nil && rtx.Agent.ContextProfile.Disabled(block)
}

func adkMessagesForMain(rtx *engine.RuntimeContext) ([]adk.Message, error) {
	if rtx == nil {
		return nil, fmt.Errorf("wfexec: nil runtime context")
	}
	cur := strings.TrimSpace(workflow.ComposeUserPrompt(rtx.WorkflowMeta, rtx.EffectiveUserPrompt()))
	if cur == "" {
		return nil, fmt.Errorf("wfexec: empty user prompt")
	}
	var msgs []adk.Message
	if turns := rtx.TranscriptReplayTurns; turns != nil {
		msgs = transcriptTurnsToADKMessages(turns)
	}
	if rm := recallUserMessageFromPromptData(rtx); rm != nil {
		msgs = append(msgs, rm)
	}
	mediaPaths := normalizeMediaPaths(rtx.EffectiveInboundMediaPaths())
	if len(mediaPaths) > 0 {
		slog.Info("wfexec.adk_main.inbound_media.received",
			"agent_type", runtimeAgentType(rtx),
			"correlation_id", strings.TrimSpace(rtx.CorrelationID),
			"count", len(mediaPaths),
			"workspace", strings.TrimSpace(rtx.EffectiveWorkspacePath()),
			"paths", mediaPaths,
		)
	}
	if len(mediaPaths) > 0 {
		mediaPaths = materializeInboundMediaToWorkspace(rtx, mediaPaths)
		slog.Info("wfexec.adk_main.inbound_media.materialized",
			"agent_type", runtimeAgentType(rtx),
			"correlation_id", strings.TrimSpace(rtx.CorrelationID),
			"count", len(mediaPaths),
			"paths", mediaPaths,
		)
	}
	if len(mediaPaths) == 0 {
		msgs = append(msgs, schema.UserMessage(cur))
		return msgs, nil
	}
	if !runtimeModelSupportsImageInput(rtx) {
		msgs = append(msgs, schema.UserMessage(injectMediaPathsPrompt(cur, mediaPaths)))
		return msgs, nil
	}
	userMsg, attached := buildUserMessageWithInlineImages(cur, mediaPaths)
	if attached == 0 {
		msgs = append(msgs, schema.UserMessage(injectMediaPathsPrompt(cur, mediaPaths)))
		return msgs, nil
	}
	msgs = append(msgs, userMsg)
	return msgs, nil
}

func materializeInboundMediaToWorkspace(rtx *engine.RuntimeContext, mediaPaths []string) []string {
	workspace := strings.TrimSpace(rtx.EffectiveWorkspacePath())
	if workspace == "" || len(mediaPaths) == 0 {
		if len(mediaPaths) > 0 {
			slog.Warn("wfexec.adk_main.inbound_materialize.skip_empty_workspace",
				"agent_type", runtimeAgentType(rtx),
				"correlation_id", strings.TrimSpace(rtx.CorrelationID),
				"count", len(mediaPaths),
			)
		}
		return mediaPaths
	}
	inboundDir := filepath.Join(workspace, "inbound")
	if err := os.MkdirAll(inboundDir, 0o755); err != nil {
		slog.Warn("wfexec.adk_main.inbound_materialize.mkdir", "dir", inboundDir, "err", err)
		return mediaPaths
	}
	slog.Info("wfexec.adk_main.inbound_materialize.begin",
		"agent_type", runtimeAgentType(rtx),
		"correlation_id", strings.TrimSpace(rtx.CorrelationID),
		"workspace", workspace,
		"inbound_dir", inboundDir,
		"count", len(mediaPaths),
	)
	out := make([]string, 0, len(mediaPaths))
	for i, loc := range mediaPaths {
		dst, ok := materializeOneInboundMedia(loc, inboundDir, i)
		if ok {
			slog.Info("wfexec.adk_main.inbound_materialize.copied",
				"src", loc,
				"dst", dst,
			)
			out = append(out, dst)
			continue
		}
		slog.Info("wfexec.adk_main.inbound_materialize.keep_original",
			"src", loc,
		)
		out = append(out, loc)
	}
	return out
}

func materializeOneInboundMedia(src, inboundDir string, idx int) (string, bool) {
	src = strings.TrimSpace(src)
	if src == "" {
		return "", false
	}
	low := strings.ToLower(src)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		return "", false
	}
	info, err := os.Stat(src)
	if err != nil || info.IsDir() {
		if err != nil {
			slog.Warn("wfexec.adk_main.inbound_materialize.stat", "src", src, "err", err)
		}
		return "", false
	}
	name := filepath.Base(src)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = fmt.Sprintf("attachment-%d", idx+1)
	}
	dst := nextAvailableInboundPath(filepath.Join(inboundDir, name))
	if err := copyFile(src, dst); err != nil {
		slog.Warn("wfexec.adk_main.inbound_materialize.copy", "src", src, "dst", dst, "err", err)
		return "", false
	}
	return dst, true
}

func nextAvailableInboundPath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		cand := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(cand); os.IsNotExist(err) {
			return cand
		}
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	return closeErr
}

func normalizeMediaPaths(paths []string) []string {
	if len(paths) == 0 {
		return nil
	}
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

func injectMediaPathsPrompt(userPrompt string, mediaPaths []string) string {
	if len(mediaPaths) == 0 {
		return userPrompt
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(userPrompt))
	b.WriteString("\n\n## Context attachment\n\n")
	for _, p := range mediaPaths {
		b.WriteString("- ")
		b.WriteString(p)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

func runtimeModelSupportsImageInput(rtx *engine.RuntimeContext) bool {
	prof, err := runtimeModelProfile(rtx)
	if err != nil {
		return false
	}
	return modelSupportsImageInput(prof)
}

func modelSupportsImageInput(prof config.ModelProfile) bool {
	provider := strings.ToLower(strings.TrimSpace(prof.Provider))
	model := strings.ToLower(strings.TrimSpace(prof.DefaultModel))
	switch provider {
	case "gemini", "claude":
		return true
	case "openai", "openai_compatible", "moonshot", "openrouter":
		return strings.Contains(model, "gpt-4o") ||
			strings.Contains(model, "gpt-4.1") ||
			strings.Contains(model, "gpt-5") ||
			strings.Contains(model, "claude-3") ||
			strings.Contains(model, "claude-4") ||
			strings.Contains(model, "gemini") ||
			strings.Contains(model, "vision") ||
			strings.Contains(model, "vl")
	case "qwen", "deepseek", "ark":
		return strings.Contains(model, "vision") || strings.Contains(model, "vl")
	default:
		return strings.Contains(model, "vision") || strings.Contains(model, "vl")
	}
}

func buildUserMessageWithInlineImages(userPrompt string, mediaPaths []string) (adk.Message, int) {
	parts := []schema.MessageInputPart{
		{
			Type: schema.ChatMessagePartTypeText,
			Text: injectMediaPathsPrompt(userPrompt, mediaPaths),
		},
	}
	attached := 0
	for _, loc := range mediaPaths {
		if attached >= maxInlineImageCount {
			break
		}
		imgPart, ok, err := imageInputPartFromLocator(loc)
		if err != nil {
			slog.Warn("wfexec.adk_main.inline_image.skip", "loc", loc, "err", err)
			continue
		}
		if !ok {
			continue
		}
		parts = append(parts, imgPart)
		attached++
	}
	return &schema.Message{
		Role:                  schema.User,
		UserInputMultiContent: parts,
	}, attached
}

func imageInputPartFromLocator(loc string) (schema.MessageInputPart, bool, error) {
	loc = strings.TrimSpace(loc)
	if loc == "" {
		return schema.MessageInputPart{}, false, nil
	}
	low := strings.ToLower(loc)
	if strings.HasPrefix(low, "http://") || strings.HasPrefix(low, "https://") {
		mimeType := imageMIMEByExt(loc)
		if !strings.HasPrefix(mimeType, "image/") {
			return schema.MessageInputPart{}, false, nil
		}
		url := loc
		return schema.MessageInputPart{
			Type: schema.ChatMessagePartTypeImageURL,
			Image: &schema.MessageInputImage{
				MessagePartCommon: schema.MessagePartCommon{
					URL:      &url,
					MIMEType: mimeType,
				},
				Detail: schema.ImageURLDetailAuto,
			},
		}, true, nil
	}
	mimeType := imageMIMEByExt(loc)
	if !strings.HasPrefix(mimeType, "image/") {
		return schema.MessageInputPart{}, false, nil
	}
	b, err := os.ReadFile(loc)
	if err != nil {
		return schema.MessageInputPart{}, false, err
	}
	if len(b) == 0 {
		return schema.MessageInputPart{}, false, nil
	}
	if len(b) > maxInlineImageBytes {
		return schema.MessageInputPart{}, false, fmt.Errorf("image too large: %d > %d", len(b), maxInlineImageBytes)
	}
	encoded := base64.StdEncoding.EncodeToString(b)
	return schema.MessageInputPart{
		Type: schema.ChatMessagePartTypeImageURL,
		Image: &schema.MessageInputImage{
			MessagePartCommon: schema.MessagePartCommon{
				Base64Data: &encoded,
				MIMEType:   mimeType,
			},
			Detail: schema.ImageURLDetailAuto,
		},
	}, true, nil
}

func imageMIMEByExt(loc string) string {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(loc)))
	switch ext {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".bmp":
		return "image/bmp"
	default:
		mt := strings.TrimSpace(mime.TypeByExtension(ext))
		if strings.HasPrefix(mt, "image/") {
			return mt
		}
		return ""
	}
}

func recallUserMessageFromPromptData(rtx *engine.RuntimeContext) adk.Message {
	if rtx == nil {
		return nil
	}
	if contextDisabled(rtx, "memory_recall") {
		return nil
	}
	raw, ok := rtx.PromptTemplateRaw("MemoryRecall")
	if !ok || raw == nil {
		return nil
	}
	body := strings.TrimSpace(promptDataString(raw))
	if body == "" {
		return nil
	}
	// load_memory_snapshot fills MemoryRecall via composeMemoryRecallPrompt (SQLite hits first, optional path digest).
	return schema.UserMessage(body)
}

func debugLogADKMainModelInput(ctx context.Context, rtx *engine.RuntimeContext, systemPrompt string, msgs []adk.Message) {
	if ctx == nil {
		ctx = context.Background()
	}
	forceInfo := os.Getenv("ONECLAW_VERBOSE_PROMPT") == "1"
	if !forceInfo && !slog.Default().Enabled(ctx, slog.LevelDebug) {
		return
	}
	agentType := ""
	corr := ""
	if rtx != nil {
		corr = rtx.CorrelationID
		if rtx.Agent != nil {
			agentType = rtx.Agent.AgentType
		}
	}
	log := slog.DebugContext
	if forceInfo {
		log = slog.InfoContext
	}
	log(ctx, "wfexec.adk_main.system_prompt",
		"agent_type", agentType,
		"correlation_id", corr,
		"chars", len(systemPrompt),
		"text", systemPrompt,
	)
	var b strings.Builder
	for i, m := range msgs {
		if m == nil {
			continue
		}
		if i > 0 {
			b.WriteString("\n--- msg ---\n")
		}
		b.WriteString(string(m.Role))
		b.WriteString(": ")
		b.WriteString(m.Content)
	}
	log(ctx, "wfexec.adk_main.chat_messages",
		"agent_type", agentType,
		"correlation_id", corr,
		"count", len(msgs),
		"text", b.String(),
	)
}

func promptDataString(v any) string {
	if v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return strings.TrimSpace(fmt.Sprint(s))
	}
}

func runtimeAgentType(rtx *engine.RuntimeContext) string {
	if rtx == nil {
		return ""
	}
	if rtx.Agent != nil && strings.TrimSpace(rtx.Agent.AgentType) != "" {
		return strings.TrimSpace(rtx.Agent.AgentType)
	}
	return strings.TrimSpace(rtx.Turn.AgentID)
}

func recordSummaryTranscript(rtx *engine.RuntimeContext) bool {
	return rtx != nil && workflow.TranscriptSummaryMode(rtx.WorkflowMeta)
}

func logToolActivity(ctx context.Context, rtx *engine.RuntimeContext, msg *schema.Message, seenCalls, seenResults map[string]bool) {
	if msg == nil {
		return
	}
	agentType := runtimeAgentType(rtx)
	corr := ""
	if rtx != nil {
		corr = strings.TrimSpace(rtx.CorrelationID)
	}
	for _, tc := range msg.ToolCalls {
		id := strings.TrimSpace(tc.ID)
		if id != "" && seenCalls[id] {
			continue
		}
		if id != "" {
			seenCalls[id] = true
		}
		slog.InfoContext(ctx, "wfexec.llm.tool_call",
			"agent_type", agentType,
			"correlation_id", corr,
			"tool_call_id", id,
			"tool_name", strings.TrimSpace(tc.Function.Name),
			"arguments_len", len(strings.TrimSpace(tc.Function.Arguments)),
		)
		if rtx != nil {
			args := strings.TrimSpace(tc.Function.Arguments)
			if len(args) > 800 {
				args = args[:800] + "…"
			}
			appendRunJournalEntry(rtx, "tool_call", map[string]any{
				"tool_call_id": id,
				"tool_name":    strings.TrimSpace(tc.Function.Name),
				"arguments":    args,
			})
		}
	}
	if msg.Role != schema.Tool {
		return
	}
	key := strings.TrimSpace(msg.ToolCallID)
	if key == "" {
		key = strings.TrimSpace(msg.ToolName)
	}
	if key != "" && seenResults[key] {
		return
	}
	if key != "" {
		seenResults[key] = true
	}
	slog.InfoContext(ctx, "wfexec.llm.tool_result",
		"agent_type", agentType,
		"correlation_id", corr,
		"tool_call_id", strings.TrimSpace(msg.ToolCallID),
		"tool_name", strings.TrimSpace(msg.ToolName),
		"content_len", len(strings.TrimSpace(msg.Content)),
	)
	if rtx != nil {
		body := strings.TrimSpace(msg.Content)
		if len(body) > 1200 {
			body = body[:1200] + "…"
		}
		appendRunJournalEntry(rtx, "tool_result", map[string]any{
			"tool_call_id": strings.TrimSpace(msg.ToolCallID),
			"tool_name":    strings.TrimSpace(msg.ToolName),
			"content":      body,
		})
	}
}

func transcriptTurnsToADKMessages(turns []session.TranscriptTurn) []adk.Message {
	msgs := make([]adk.Message, 0, len(turns))
	for _, t := range turns {
		role := strings.ToLower(strings.TrimSpace(t.Role))
		content := strings.TrimSpace(t.Content)
		if content == "" {
			continue
		}
		switch role {
		case "user":
			msgs = append(msgs, schema.UserMessage(content))
		case "assistant":
			msgs = append(msgs, schema.AssistantMessage(content, nil))
		default:
			continue
		}
	}
	return msgs
}

func handleOnRespond(_ context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	rtx := env.Runtime
	if strings.TrimSpace(in.Text) != "" {
		rtx.SetAssistant(strings.TrimSpace(in.Text))
	}
	rtx.SetSawOnRespond(true)
	if strings.TrimSpace(rtx.Assistant) == "" {
		return workflow.WorkflowNodeResult{}, nil
	}
	assistOut := strings.TrimSpace(rtx.Assistant)
	transcriptBody := assistOut
	if recordSummaryTranscript(rtx) {
		transcriptBody = fmt.Sprintf("[%s] completed · reply_len=%d", runtimeAgentType(rtx), len(assistOut))
	}
	if err := session.AppendTranscriptTurn(rtx.EffectiveSessionRoot(), runtimeAgentType(rtx), session.TranscriptTurn{
		Ts: time.Now().UTC(), Role: "assistant", Content: transcriptBody,
	}); err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	appendRunJournalEntry(rtx, "assistant_message", map[string]any{"content": rtx.Assistant})
	if rtx.PostAssistantRespond != nil {
		c := rtx.GoCtx
		if c == nil {
			c = context.Background()
		}
		if err := rtx.PostAssistantRespond(c, rtx.Assistant); err != nil {
			return workflow.WorkflowNodeResult{}, err
		}
	}
	rtx.EmitNodeOutput(map[string]any{
		"use":              "on_respond",
		"assistant_text":   rtx.Assistant,
		"transcript_flush": true,
	})
	return workflow.WorkflowNodeResult{Text: strings.TrimSpace(rtx.Assistant)}, nil
}

func runtimeModelProfile(rtx *engine.RuntimeContext) (config.ModelProfile, error) {
	if rtx == nil || rtx.Cfg == nil {
		return config.ModelProfile{}, nil
	}
	profile := strings.TrimSpace(rtx.EffectiveProfileID())
	modelName := strings.TrimSpace(rtx.EffectiveModelName())
	switch {
	case profile == "":
		return config.ModelProfile{}, nil
	case modelName != "":
		p, err := config.ResolveModelForTurn(rtx.Cfg, profile+"/"+modelName)
		if err != nil {
			return config.ModelProfile{}, err
		}
		return *p, nil
	default:
		// Fallback: resolve by profile id/provider and use existing cfg default model path.
		all, err := config.ResolveProfilesForCredentialKey(rtx.Cfg, profile)
		if err != nil || len(all) == 0 {
			return config.ModelProfile{}, err
		}
		return all[0], nil
	}
}
