package session

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/lengzhao/oneclaw/loop"
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
			"prompt_tokens":       end.PromptTokens,
			"completion_tokens":   end.CompletionTokens,
			"total_tokens":        end.TotalTokens,
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
	if !e.wantsLifecycle() {
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
					rec := map[string]any{
						"record":         "turn_first_model_request",
						"turn_id":        turnID,
						"correlation_id": corrID,
						"agent_id":       agentID,
						"message_count":  len(reqMsgs),
						"messages":       json.RawMessage(full),
					}
					e.appendExecutionRecord(c, rec)
				}
			}
			startData := map[string]any{
				"record":                 "model_step_start",
				"turn_id":                turnID,
				"correlation_id":         corrID,
				"agent_id":               agentID,
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
			e.appendExecutionRecord(c, startData)
		},
		OnModelStepEnd: func(c context.Context, step int, end loop.ModelStepEndInfo) {
			rec := modelStepEndNotifyData(step, end)
			rec["record"] = "model_step_end"
			rec["turn_id"] = turnID
			rec["correlation_id"] = corrID
			rec["agent_id"] = agentID
			e.appendExecutionRecord(c, rec)
		},
		OnToolStart: func(c context.Context, modelStep int, toolUseID, toolName, argsPreview string) {
			e.appendExecutionRecord(c, map[string]any{
				"record":         "tool_call_start",
				"turn_id":        turnID,
				"correlation_id": corrID,
				"agent_id":       agentID,
				"model_step":     modelStep,
				"tool_use_id":    toolUseID,
				"name":           toolName,
				"args_preview":   argsPreview,
			})
		},
	}
}

func (e *Engine) nestedLoopLifecycle(parentTurnID, parentCorrID, childTurnID, childRunID, nestedAgentID, parentAgentID string, depth int) *loop.LifecycleCallbacks {
	if !e.wantsLifecycle() {
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
					e.appendExecutionRecord(c, map[string]any{
						"record":           "turn_first_model_request",
						"turn_id":          childTurnID,
						"run_id":           childRunID,
						"correlation_id":   parentCorrID,
						"agent_id":         subID,
						"parent_agent_id":  pID,
						"parent_run_id":    parentTurnID,
						"message_count":    len(reqMsgs),
						"messages":         json.RawMessage(full),
						"subagent_depth":   depth,
					})
				}
			}
			startData := map[string]any{
				"record":                 "model_step_start",
				"turn_id":                childTurnID,
				"run_id":                 childRunID,
				"correlation_id":         parentCorrID,
				"agent_id":               subID,
				"parent_agent_id":        pID,
				"parent_run_id":          parentTurnID,
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
			e.appendExecutionRecord(c, startData)
		},
		OnModelStepEnd: func(c context.Context, step int, end loop.ModelStepEndInfo) {
			rec := modelStepEndNotifyData(step, end)
			rec["record"] = "model_step_end"
			rec["turn_id"] = childTurnID
			rec["run_id"] = childRunID
			rec["correlation_id"] = parentCorrID
			rec["agent_id"] = subID
			rec["parent_agent_id"] = pID
			rec["parent_run_id"] = parentTurnID
			rec["subagent_depth"] = depth
			e.appendExecutionRecord(c, rec)
		},
		OnToolStart: func(c context.Context, modelStep int, toolUseID, toolName, argsPreview string) {
			e.appendExecutionRecord(c, map[string]any{
				"record":           "tool_call_start",
				"turn_id":          childTurnID,
				"run_id":           childRunID,
				"correlation_id":   parentCorrID,
				"agent_id":         subID,
				"parent_agent_id":  pID,
				"parent_run_id":    parentTurnID,
				"model_step":       modelStep,
				"tool_use_id":      toolUseID,
				"name":             toolName,
				"args_preview":     argsPreview,
				"subagent_depth":   depth,
			})
		},
	}
}
