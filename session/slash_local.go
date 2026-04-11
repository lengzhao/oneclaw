package session

import (
	"fmt"
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

// trySlashLocalTurn handles built-in slash commands without calling the model.
func (e *Engine) trySlashLocalTurn(in bus.InboundMessage) (reply string, ok bool) {
	cmd, args, slash := ParseLeadingSlash(in.Content)
	if !slash {
		return "", false
	}
	switch cmd {
	case "help", "h", "?", "commands":
		return slashHelpText(), true
	case "model":
		return fmt.Sprintf("当前模型（Current model）: %s", strings.TrimSpace(e.Model)), true
	case "session":
		return e.slashSession(in, args), true
	case "status", "st":
		return e.slashStatus(in), true
	case "paths", "memory-paths", "mempaths":
		return e.slashPaths(), true
	case "recall":
		return e.slashRecall(args), true
	default:
		return "", false
	}
}

func slashHelpText() string {
	return strings.TrimSpace(`
内置斜杠命令（不经过模型）：
  /help, /h, /?, /commands — 显示本帮助
  /model — 显示当前模型名
  /session — 会话 ID 与工作目录（简版）
  /session full — 同 /status，完整会话与运行参数
  /status, /st — 会话 ID、CWD、模型、步数/Token、转录路径、消息条数、recall 统计等
  /paths, /memory-paths — 当前记忆目录布局（Layout）与各根路径
  /recall reset — 清空本会话 recall 去重状态并尝试持久化（见 /recall 说明）

其它以 / 开头的输入仍会交给模型处理。CLI 下仍可用 /exit 退出（由终端层处理，不进引擎）。
`)
}
