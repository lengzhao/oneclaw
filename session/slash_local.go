package session

import (
	"fmt"
	"strings"

	"github.com/lengzhao/clawbridge/bus"
)

// trySlashLocalTurn handles built-in slash commands without calling the model.
func (e *Engine) trySlashLocalTurn(in bus.InboundMessage) (reply string, ok bool) {
	cmd, _, slash := ParseLeadingSlash(in.Content)
	if !slash {
		return "", false
	}
	switch cmd {
	case "help", "h", "?", "commands":
		return slashHelpText(), true
	case "model":
		return fmt.Sprintf("当前模型（Current model）: %s", strings.TrimSpace(e.Model)), true
	case "session":
		return fmt.Sprintf("会话 ID: %s\n工作目录: %s", e.SessionID, e.CWD), true
	default:
		return "", false
	}
}

func slashHelpText() string {
	return strings.TrimSpace(`
内置斜杠命令（不经过模型）：
  /help, /h, /?, /commands — 显示本帮助
  /model — 显示当前模型名
  /session — 显示会话 ID 与工作目录

其它以 / 开头的输入仍会交给模型处理。CLI 下仍可用 /exit 退出（由终端层处理，不进引擎）。
`)
}
