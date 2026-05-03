package wfexec

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/lengzhao/oneclaw/engine"
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
	// Instruction already assembled in preturn.Bundle before workflow (workflows-spec §6).
	return nil
}

func handleLoadMemorySnapshot(*engine.RuntimeContext) error { return nil }

func handleFilterTools(*engine.RuntimeContext) error { return nil }

func handleNoop(*engine.RuntimeContext) error { return nil }

func handleADKMain(rtx *engine.RuntimeContext) error {
	if rtx.ChatAgent == nil {
		return fmt.Errorf("wfexec: adk_main: ChatAgent not configured")
	}
	input := &adk.AgentInput{
		Messages: []adk.Message{schema.UserMessage(rtx.EffectiveUserPrompt())},
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
			msg := ev.Output.MessageOutput.Message
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
	rtx.Assistant = strings.TrimSpace(strings.Join(chunks, "\n"))
	return nil
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
