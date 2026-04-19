package session

import (
	"context"
	"encoding/json"

	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/lengzhao/oneclaw/tools"
)

const denySendMessageOnScheduleSyntheticReason = "此回合为定时任务注入：正文已作为用户消息出现在当前会话。若仅通知自己，请用普通助手回复，勿再对同一会话调用 send_message。若要通知其他人/渠道，请在 send_message 中显式填写 client_id（或旧字段 source）、session_id（或旧 session_key）、to_user_id、peer_kind 等覆盖字段。"

// denySendMessageOnSyntheticScheduleTurn blocks send_message on host-injected schedule turns only when
// the tool would target the same client/thread/user as the current turn (duplicate delivery).
// Cross-target sends (e.g. another clawbridge client id) remain allowed.
func denySendMessageOnSyntheticScheduleTurn(tctx *toolctx.Context, toolName string, toolInput json.RawMessage) (deny bool, reason string) {
	if toolName != "send_message" || tctx == nil {
		return false, ""
	}
	if !schedule.IsSyntheticScheduleInbound(&tctx.TurnInbound) {
		return false, ""
	}
	a := SendMessageToolRoutingFromJSON(toolInput)
	if SendMessageTargetOverridesTurn(tctx.TurnInbound, a) {
		return false, ""
	}
	return true, denySendMessageOnScheduleSyntheticReason
}

// DefaultCanUseToolWithScheduleGate returns a [tools.CanUseTool] that blocks duplicate send_message delivery on synthetic schedule turns (see denySendMessageOnSyntheticScheduleTurn).
func DefaultCanUseToolWithScheduleGate() tools.CanUseTool {
	return func(ctx context.Context, name string, input json.RawMessage, tctx *toolctx.Context) (bool, string) {
		_ = ctx
		if deny, reason := denySendMessageOnSyntheticScheduleTurn(tctx, name, input); deny {
			return false, reason
		}
		return true, ""
	}
}
