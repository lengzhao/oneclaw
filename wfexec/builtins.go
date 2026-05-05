package wfexec

import (
	"context"
	"fmt"
	"log/slog"
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

// RegisterBuiltins registers handlers for each entry in workflow.BuiltinUses.
func RegisterBuiltins(r *Registry) error {
	if r == nil {
		return fmt.Errorf("wfexec: nil registry")
	}
	byUse := map[string]Handler{
		"on_receive":           handleOnReceive,
		"llm":                  handleLLM,
		"on_respond":           handleOnRespond,
		"agent_task":           handleAgentTask,
		"retrieve_context":     handlePassthroughTextNode,
		"command":              handlePassthroughTextNode,
		"tool_call":            handlePassthroughTextNode,
		"noop":                 handleNoop,
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
	cur := strings.TrimSpace(rtx.EffectiveUserPrompt())
	if cur == "" {
		return fmt.Errorf("wfexec: adk_main: empty user prompt")
	}
	if !rtx.UserTurnAppended {
		if err := session.AppendTranscriptTurn(rtx.EffectiveSessionRoot(), runtimeAgentType(rtx), session.TranscriptTurn{
			Ts: time.Now().UTC(), Role: "user", Content: cur,
		}); err != nil {
			return fmt.Errorf("wfexec: adk_main: append user transcript: %w", err)
		}
		rtx.UserTurnAppended = true
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
		"use":            "llm",
		"assistant_text": rtx.Assistant,
		"user_prompt":    rtx.UserPrompt,
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
	cur := strings.TrimSpace(rtx.EffectiveUserPrompt())
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
	msgs = append(msgs, schema.UserMessage(cur))
	return msgs, nil
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
	if err := session.AppendTranscriptTurn(rtx.EffectiveSessionRoot(), runtimeAgentType(rtx), session.TranscriptTurn{
		Ts: time.Now().UTC(), Role: "assistant", Content: rtx.Assistant,
	}); err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	if rtx.PostAssistantRespond != nil {
		c := rtx.GoCtx
		if c == nil {
			c = context.Background()
		}
		if err := rtx.PostAssistantRespond(c, rtx.Assistant); err != nil {
			return workflow.WorkflowNodeResult{}, err
		}
	}
	if rtx.DelegationDepth == 0 {
		c := rtx.GoCtx
		if c == nil {
			c = context.Background()
		}
		if err := extractStructuredMemoryFromMainTurn(rtx); err != nil {
			slog.WarnContext(c, "wfexec.structuredmem.extract_failed",
				"agent_type", runtimeAgentType(rtx),
				"correlation_id", strings.TrimSpace(rtx.CorrelationID),
				"err", err,
			)
		}
	}
	rtx.EmitNodeOutput(map[string]any{
		"use":              "on_respond",
		"assistant_text":   rtx.Assistant,
		"transcript_flush": true,
	})
	return workflow.WorkflowNodeResult{Text: strings.TrimSpace(rtx.Assistant)}, nil
}

func extractStructuredMemoryFromMainTurn(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	c := rtx.GoCtx
	if c == nil {
		c = context.Background()
	}
	assistant := strings.TrimSpace(rtx.Assistant)
	user := strings.TrimSpace(rtx.EffectiveUserPrompt())
	if user == "" || assistant == "" {
		return nil
	}
	prof, err := runtimeModelProfile(rtx)
	if err != nil {
		return err
	}
	if strings.TrimSpace(prof.ID) == "" && strings.TrimSpace(prof.Provider) == "" {
		return nil
	}
	dialog := "User: " + user + "\nAssistant: " + assistant
	agentID := runtimeAgentType(rtx)
	return structuredmem.AppendExtractJournal(
		c,
		rtx.EffectiveInstructionRoot(),
		dialog,
		prof,
		rtx.EffectiveSessionSegment(),
		agentID,
		time.Now().UTC(),
	)
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
