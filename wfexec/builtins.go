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

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/preturn"
	"github.com/lengzhao/oneclaw/session"
)

// RegisterPhase3Builtins wires handlers for workflow.Phase3Uses.
func RegisterPhase3Builtins(r *Registry) error {
	if r == nil {
		return fmt.Errorf("wfexec: nil registry")
	}
	for _, pair := range []struct {
		use string
		h   Handler
	}{
		{"on_receive", handleOnReceive},
		{"load_prompt_md", handleLoadPromptMD},
		{"load_memory_snapshot", handleLoadMemorySnapshot},
		{"list_skills", handleListSkills},
		{"list_tasks", handleListTasks},
		{"load_transcript", handleLoadTranscript},
		{"filter_tools", handleFilterTools},
		{"adk_main", handleADKMain},
		{"on_respond", handleOnRespond},
		{"agent", handleAgent},
		{"noop", handleNoop},
	} {
		if err := r.Register(pair.use, pair.h); err != nil {
			return err
		}
	}
	return nil
}

func handleOnReceive(rtx *engine.RuntimeContext) error {
	if strings.TrimSpace(rtx.EffectiveUserPrompt()) == "" {
		return fmt.Errorf("wfexec: on_receive: empty user prompt")
	}
	return nil
}

func handleLoadPromptMD(*engine.RuntimeContext) error {
	// Prompt fragments are assembled at adk_main via RenderMainAgentPrompt + workflow-filled PromptTemplateData.
	return nil
}

func handleLoadMemorySnapshot(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	if rtx.Agent != nil && rtx.Agent.AgentType == "memory_extractor" {
		return nil
	}
	budget := preturn.CoalesceBudget(preturn.DefaultBudget())
	block := preturn.MemoryRecallSection(rtx.EffectiveInstructionRoot(), budget)
	ensurePromptData(rtx)["MemoryRecall"] = block
	return nil
}

func handleListSkills(rtx *engine.RuntimeContext) error {
	if rtx == nil {
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
	ensurePromptData(rtx)["SkillsIndex"] = s
	return nil
}

func handleListTasks(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	p := filepath.Join(rtx.EffectiveInstructionRoot(), "todo.json")
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			ensurePromptData(rtx)["Tasks"] = "(no todo.json — use the `todo` tool with action=list after adding tasks)"
			return nil
		}
		return fmt.Errorf("wfexec: list_tasks: %w", err)
	}
	ensurePromptData(rtx)["Tasks"] = strings.TrimSpace(string(b))
	return nil
}

func handleLoadTranscript(rtx *engine.RuntimeContext) error {
	if rtx == nil {
		return nil
	}
	turns, err := session.LoadTranscriptTurns(rtx.EffectiveSessionRoot())
	if err != nil {
		return fmt.Errorf("wfexec: load_transcript: %w", err)
	}
	turns = session.TrimTranscriptTail(turns, session.DefaultTranscriptTurnLimit)
	rtx.TranscriptReplayTurns = turns
	return nil
}

func ensurePromptData(rtx *engine.RuntimeContext) map[string]any {
	if rtx.PromptTemplateData == nil {
		rtx.PromptTemplateData = make(map[string]any)
	}
	return rtx.PromptTemplateData
}

func handleFilterTools(*engine.RuntimeContext) error { return nil }

func handleNoop(*engine.RuntimeContext) error { return nil }

func handleADKMain(rtx *engine.RuntimeContext) error {
	if rtx.ChatAgent == nil {
		return fmt.Errorf("wfexec: adk_main: ChatAgent not configured")
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
	if err := session.AppendTranscriptTurn(rtx.EffectiveSessionRoot(), session.TranscriptTurn{
		Ts: time.Now().UTC(), Role: "user", Content: cur,
	}); err != nil {
		return fmt.Errorf("wfexec: adk_main: append user transcript: %w", err)
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
			if role == schema.Tool {
				continue
			}
			c := strings.TrimSpace(msg.Content)
			if c == "" {
				continue
			}
			chunks = append(chunks, c)
			if rtx.Stdout != nil {
				fmt.Fprintln(rtx.Stdout, c)
			}
			if rtx.OnAssistantChunk != nil {
				rtx.OnAssistantChunk(c)
			}
		}
	}
	// Join assistant MessageOutputs (intermediate model text included). Tool result outputs (Role tool) are omitted.
	if len(chunks) == 0 {
		rtx.Assistant = ""
	} else {
		rtx.Assistant = strings.TrimSpace(strings.Join(chunks, "\n"))
	}
	return nil
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
	if rtx == nil || rtx.PromptTemplateData == nil {
		return nil
	}
	raw, ok := rtx.PromptTemplateData["MemoryRecall"]
	if !ok || raw == nil {
		return nil
	}
	body := strings.TrimSpace(promptDataString(raw))
	if body == "" {
		return nil
	}
	// load_memory_snapshot fills MemoryRecall from [preturn.MemoryRecallSection], which already starts with "## Memory recall".
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

func handleOnRespond(rtx *engine.RuntimeContext) error {
	rtx.SawOnRespond = true
	if strings.TrimSpace(rtx.Assistant) == "" {
		return nil
	}
	if err := session.AppendTranscriptTurn(rtx.EffectiveSessionRoot(), session.TranscriptTurn{
		Ts: time.Now().UTC(), Role: "assistant", Content: rtx.Assistant,
	}); err != nil {
		return err
	}
	if rtx.PostAssistantRespond != nil {
		c := rtx.GoCtx
		if c == nil {
			c = context.Background()
		}
		return rtx.PostAssistantRespond(c, rtx.Assistant)
	}
	return nil
}
