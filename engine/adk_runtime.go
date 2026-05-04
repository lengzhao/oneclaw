package engine

import "github.com/cloudwego/eino/adk"

// SetAssistant replaces the last main-agent assistant text (adk_main).
func (rtx *RuntimeContext) SetAssistant(text string) {
	if rtx == nil {
		return
	}
	rtx.Assistant = text
}

// SetSawOnRespond records whether transcript flush was handled by on_respond.
func (rtx *RuntimeContext) SetSawOnRespond(v bool) {
	if rtx == nil {
		return
	}
	rtx.SawOnRespond = v
}

// SetChatAgent replaces the runnable chat agent (e.g. after instruction rebuild).
func (rtx *RuntimeContext) SetChatAgent(agent *adk.ChatModelAgent) {
	if rtx == nil {
		return
	}
	rtx.ChatAgent = agent
}
