package wfexec

import (
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

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
		{"noop", handleNoop},
	} {
		if err := r.Register(pair.use, pair.h); err != nil {
			return err
		}
	}
	return nil
}

func handleOnReceive(rtx *RuntimeContext) error {
	if strings.TrimSpace(rtx.UserPrompt) == "" {
		return fmt.Errorf("wfexec: on_receive: empty user prompt")
	}
	return nil
}

func handleLoadPromptMD(*RuntimeContext) error {
	// Instruction already assembled in preturn.Bundle before workflow (workflows-spec §6).
	return nil
}

func handleLoadMemorySnapshot(*RuntimeContext) error { return nil }

func handleFilterTools(*RuntimeContext) error { return nil }

func handleNoop(*RuntimeContext) error { return nil }

func handleADKMain(rtx *RuntimeContext) error {
	if rtx.ChatAgent == nil {
		return fmt.Errorf("wfexec: adk_main: ChatAgent not configured")
	}
	input := &adk.AgentInput{
		Messages: []adk.Message{schema.UserMessage(rtx.UserPrompt)},
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

func handleOnRespond(rtx *RuntimeContext) error {
	rtx.SawOnRespond = true
	if strings.TrimSpace(rtx.Assistant) == "" {
		return nil
	}
	return session.AppendTranscriptTurn(rtx.SessionRoot, session.TranscriptTurn{
		Ts: time.Now().UTC(), Role: "assistant", Content: rtx.Assistant,
	})
}
