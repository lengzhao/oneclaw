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
	case "reset":
		return e.slashReset(args), true
	case "stop":
		if strings.TrimSpace(args) != "" {
			return fmt.Sprintf("用法：仅发送 /stop（不要附加参数）。收到：%q", strings.TrimSpace(args)), true
		}
		return strings.TrimSpace(`
已处理 /stop：若本会话当前轮次已在 worker 上执行（模型或工具循环），入站侧已尝试取消该轮次。
若当前没有正在执行的轮次、或下一条消息仍在队列中尚未开始，则取消无效果（排队中的任务需等轮到它时才会运行或可被下一轮 /stop 取消）。
`), true
	default:
		return "", false
	}
}

func slashHelpText() string {
	return strings.TrimSpace(`
内置斜杠命令（不经过模型）：
  /help, /h, /?, /commands — 显示本帮助
  /model — 显示当前模型名
  /session — driver client_id / bus.SessionID、路由 session_id、工作区 ID、CWD（简版）
  /session full — 同 /status，完整会话与运行参数
  /status, /st — 会话 ID、CWD、模型、步数/Token、转录路径、消息条数等
  /paths, /memory-paths — 当前记忆目录布局（Layout）与各根路径
  /reset — 清空本会话聊天转录与模型上下文（内存与磁盘）；不删除 MEMORY.md 等说明文件
  /stop — 取消本会话当前正在执行的轮次（入站线程先中止再入队；不经过模型）

其它以 / 开头的输入仍会交给模型处理。CLI 下仍可用 /exit 退出（由终端层处理，不进引擎）。
`)
}
