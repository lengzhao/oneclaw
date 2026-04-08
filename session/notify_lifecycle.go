package session

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/notify"
	"github.com/openai/openai-go"
)

func modelStepEndNotifyData(step int, end loop.ModelStepEndInfo) map[string]any {
	m := map[string]any{
		"step":             step,
		"ok":               end.OK,
		"duration_ms":      end.DurationMs,
		"tool_calls_count": end.ToolCallsCount,
	}
	if end.Model != "" {
		m["model"] = end.Model
	}
	if end.AssistantVisible != "" {
		m["assistant_visible"] = end.AssistantVisible
	}
	if end.ToolCallsJSON != "" {
		m["tool_calls_json"] = end.ToolCallsJSON
	}
	if end.FinishReason != "" {
		m["finish_reason"] = end.FinishReason
	}
	if end.PromptTokens != 0 || end.CompletionTokens != 0 || end.TotalTokens != 0 {
		m["usage"] = map[string]any{
			"prompt_tokens":     end.PromptTokens,
			"completion_tokens": end.CompletionTokens,
			"total_tokens":      end.TotalTokens,
		}
	}
	if end.Err != nil {
		m["error"] = end.Err.Error()
	}
	if end.BeforeRequestCancelled {
		m["cancel_before_request"] = true
	}
	return m
}

func (e *Engine) buildLoopLifecycle(turnID, corrID, rootAgentID string) *loop.LifecycleCallbacks {
	if !e.hasNotify() {
		return nil
	}
	agentID := strings.TrimSpace(rootAgentID)
	if agentID == "" {
		agentID = DefaultRootAgentID
	}
	return &loop.LifecycleCallbacks{
		OnModelStepStart: func(c context.Context, step, toolN int, reqMsgs []openai.ChatCompletionMessageParamUnion) {
			if step == 0 {
				full, err := loop.RequestMessagesJSONArray(reqMsgs)
				if err != nil {
					slog.Warn("session.notify.turn_first_model_request_marshal", "err", err)
				} else {
					ev0 := notify.NewEvent(notify.EventTurnFirstModelRequest, "")
					e.stampNotify(&ev0, turnID, turnID, corrID, agentID, "", "")
					ev0.Data = map[string]any{
						"message_count": len(reqMsgs),
						"messages":    json.RawMessage(full),
					}
					notify.EmitSafe(e.Notify, c, ev0)
				}
			}
			ev := notify.NewEvent(notify.EventModelStepStart, "")
			e.stampNotify(&ev, turnID, turnID, corrID, agentID, "", "")
			startData := map[string]any{
				"step":                   step,
				"tool_definitions_count": toolN,
			}
			if len(reqMsgs) > 0 {
				if lb, err := json.Marshal(reqMsgs[len(reqMsgs)-1]); err != nil {
					slog.Warn("session.notify.model_step_start_last_message", "err", err)
				} else {
					startData["last_message"] = json.RawMessage(lb)
				}
			}
			ev.Data = startData
			notify.EmitSafe(e.Notify, c, ev)
		},
		OnModelStepEnd: func(c context.Context, step int, end loop.ModelStepEndInfo) {
			ev := notify.NewEvent(notify.EventModelStepEnd, "")
			if !end.OK {
				ev.Severity = "error"
			}
			e.stampNotify(&ev, turnID, turnID, corrID, agentID, "", "")
			ev.Data = modelStepEndNotifyData(step, end)
			notify.EmitSafe(e.Notify, c, ev)
		},
		OnToolStart: func(c context.Context, modelStep int, toolUseID, toolName, argsPreview string) {
			ev := notify.NewEvent(notify.EventToolCallStart, "")
			e.stampNotify(&ev, turnID, turnID, corrID, agentID, "", "")
			ev.Data = map[string]any{
				"model_step":   modelStep,
				"tool_use_id":  toolUseID,
				"name":         toolName,
				"args_preview": argsPreview,
			}
			notify.EmitSafe(e.Notify, c, ev)
		},
	}
}

func (e *Engine) nestedLoopLifecycle(parentTurnID, parentCorrID, childTurnID, childRunID, nestedAgentID, parentAgentID string, depth int) *loop.LifecycleCallbacks {
	if !e.hasNotify() {
		return nil
	}
	pID := strings.TrimSpace(parentAgentID)
	if pID == "" {
		pID = DefaultRootAgentID
	}
	subID := strings.TrimSpace(nestedAgentID)
	return &loop.LifecycleCallbacks{
		OnModelStepStart: func(c context.Context, step, toolN int, reqMsgs []openai.ChatCompletionMessageParamUnion) {
			if step == 0 {
				full, err := loop.RequestMessagesJSONArray(reqMsgs)
				if err != nil {
					slog.Warn("session.notify.turn_first_model_request_marshal", "err", err, "subagent_depth", depth)
				} else {
					ev0 := notify.NewEvent(notify.EventTurnFirstModelRequest, "")
					e.stampNotify(&ev0, childTurnID, childRunID, parentCorrID, subID, pID, parentTurnID)
					ev0.Data = map[string]any{
						"message_count":    len(reqMsgs),
						"messages":       json.RawMessage(full),
						"subagent_depth": depth,
					}
					notify.EmitSafe(e.Notify, c, ev0)
				}
			}
			ev := notify.NewEvent(notify.EventModelStepStart, "")
			e.stampNotify(&ev, childTurnID, childRunID, parentCorrID, subID, pID, parentTurnID)
			startData := map[string]any{
				"step":                   step,
				"tool_definitions_count": toolN,
				"subagent_depth":         depth,
			}
			if len(reqMsgs) > 0 {
				if lb, err := json.Marshal(reqMsgs[len(reqMsgs)-1]); err != nil {
					slog.Warn("session.notify.model_step_start_last_message", "err", err)
				} else {
					startData["last_message"] = json.RawMessage(lb)
				}
			}
			ev.Data = startData
			notify.EmitSafe(e.Notify, c, ev)
		},
		OnModelStepEnd: func(c context.Context, step int, end loop.ModelStepEndInfo) {
			ev := notify.NewEvent(notify.EventModelStepEnd, "")
			if !end.OK {
				ev.Severity = "error"
			}
			e.stampNotify(&ev, childTurnID, childRunID, parentCorrID, subID, pID, parentTurnID)
			ev.Data = modelStepEndNotifyData(step, end)
			notify.EmitSafe(e.Notify, c, ev)
		},
		OnToolStart: func(c context.Context, modelStep int, toolUseID, toolName, argsPreview string) {
			ev := notify.NewEvent(notify.EventToolCallStart, "")
			e.stampNotify(&ev, childTurnID, childRunID, parentCorrID, subID, pID, parentTurnID)
			ev.Data = map[string]any{
				"model_step":   modelStep,
				"tool_use_id":  toolUseID,
				"name":         toolName,
				"args_preview": argsPreview,
				"subagent_depth": depth,
			}
			notify.EmitSafe(e.Notify, c, ev)
		},
	}
}
