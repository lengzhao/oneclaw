package session

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/clawbridge/bus"
	"github.com/lengzhao/oneclaw/loop"
)

func (e *Engine) slashSession(in bus.InboundMessage, args string) string {
	a := strings.ToLower(strings.TrimSpace(args))
	switch a {
	case "", "short":
		return e.slashSessionShort(in)
	case "full", "status", "info":
		return e.slashStatus(in)
	default:
		return fmt.Sprintf("未知子命令 %q。用法：/session 或 /session full（同 /status）", strings.TrimSpace(args))
	}
}

// slashSessionShort is the default /session reply: driver envelope + workspace id + cwd.
func (e *Engine) slashSessionShort(in bus.InboundMessage) string {
	var b strings.Builder
	dClient := strings.TrimSpace(in.ClientID)
	dSess := strings.TrimSpace(in.SessionID)
	route := InboundSessionKey(in)
	if dClient != "" {
		fmt.Fprintf(&b, "driver client_id: %s\n", dClient)
	} else {
		b.WriteString("driver client_id: （空）\n")
	}
	if dSess != "" {
		fmt.Fprintf(&b, "driver session_id (bus.SessionID): %s\n", dSess)
	} else {
		b.WriteString("driver session_id (bus.SessionID): （空）\n")
	}
	if route != "" {
		fmt.Fprintf(&b, "路由 session_id (send_message / OutboundMessage.To): %s\n", route)
	} else {
		b.WriteString("路由 session_id (send_message): （空）\n")
	}
	fmt.Fprintf(&b, "工作区会话 ID (勿当 send_message 目标): %s\n工作目录: %s", e.SessionID, e.CWD)
	return strings.TrimRight(b.String(), "\n")
}

func (e *Engine) slashStatus(in bus.InboundMessage) string {
	var b strings.Builder
	ch := strings.TrimSpace(in.ClientID)
	if ch != "" {
		fmt.Fprintf(&b, "driver client_id: %s\n", ch)
	} else {
		b.WriteString("driver client_id: （空）\n")
	}
	ds := strings.TrimSpace(in.SessionID)
	if ds != "" {
		fmt.Fprintf(&b, "driver session_id (bus.SessionID): %s\n", ds)
	} else {
		b.WriteString("driver session_id (bus.SessionID): （空）\n")
	}
	if rs := InboundSessionKey(in); rs != "" {
		fmt.Fprintf(&b, "路由 session_id (send_message / clawbridge): %s\n", rs)
	} else {
		b.WriteString("路由 session_id (send_message): （空）\n")
	}
	fmt.Fprintf(&b, "工作区会话 ID (StableSessionID，勿填 send_message.session_id): %s\n", e.SessionID)
	fmt.Fprintf(&b, "工作目录 CWD: %s\n", e.CWD)
	if ur := strings.TrimSpace(e.UserDataRoot); ur != "" {
		fmt.Fprintf(&b, "用户数据根 UserDataRoot: %s\n", ur)
	}
	fmt.Fprintf(&b, "WorkspaceFlat: %v\n", e.WorkspaceFlat)
	fmt.Fprintf(&b, "模型 Model: %s\n", strings.TrimSpace(e.Model))
	fmt.Fprintf(&b, "MaxSteps: %d  MaxTokens: %d\n", e.MaxSteps, e.MaxTokens)
	fmt.Fprintf(&b, "RootAgentID: %s\n", e.EffectiveRootAgentID())
	if tp := strings.TrimSpace(e.TranscriptPath); tp != "" {
		fmt.Fprintf(&b, "TranscriptPath: %s\n", tp)
	} else {
		b.WriteString("TranscriptPath: （未配置）\n")
	}
	if wp := strings.TrimSpace(e.WorkingTranscriptPath); wp != "" {
		fmt.Fprintf(&b, "WorkingTranscriptPath: %s\n", wp)
	} else {
		b.WriteString("WorkingTranscriptPath: （未配置）\n")
	}
	if e.WorkingTranscriptMaxMessages != 0 {
		fmt.Fprintf(&b, "WorkingTranscriptMaxMessages: %d\n", e.WorkingTranscriptMaxMessages)
	}
	vis := loop.ToUserVisibleMessages(e.Messages)
	fmt.Fprintf(&b, "可见 API 消息条数: %d\n", len(vis))
	fmt.Fprintf(&b, "Transcript 条数: %d\n", len(e.Transcript))
	return strings.TrimRight(b.String(), "\n")
}

func (e *Engine) slashPaths() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Sprintf("无法解析用户主目录: %v", err)
	}
	layout := e.MemoryLayout(home)
	var b strings.Builder
	fmt.Fprintf(&b, "CWD: %s\n", layout.CWD)
	fmt.Fprintf(&b, "MemoryBase: %s\n", layout.MemoryBase)
	fmt.Fprintf(&b, "User: %s\n", layout.User)
	fmt.Fprintf(&b, "Project: %s\n", layout.Project)
	fmt.Fprintf(&b, "Auto: %s\n", layout.Auto)
	b.WriteString("AgentDefault:\n")
	for _, p := range layout.AgentDefault {
		fmt.Fprintf(&b, "  %s\n", p)
	}
	fmt.Fprintf(&b, "Entrypoint: %s\n", layout.EntrypointName)
	fmt.Fprintf(&b, "HostUserData: %v\n", layout.HostUserData)
	if layout.InstructionRoot != "" {
		fmt.Fprintf(&b, "InstructionRoot: %s\n", layout.InstructionRoot)
	}
	fmt.Fprintf(&b, "DotOrDataRoot: %s\n", layout.DotOrDataRoot())
	ep := filepath.Join(layout.Project, layout.EntrypointName)
	if layout.InstructionRoot != "" {
		ep = filepath.Join(layout.InstructionRoot, layout.EntrypointName)
	}
	fmt.Fprintf(&b, "项目记忆入口（若存在）: %s\n", ep)
	return strings.TrimRight(b.String(), "\n")
}

// slashReset clears in-memory chat state and persisted slim/working transcripts for this session.
// Does not delete MEMORY.md or other instruction files on disk.
func (e *Engine) slashReset(args string) string {
	if strings.TrimSpace(args) != "" {
		return fmt.Sprintf("用法：仅发送 /reset（不要附加参数）。收到：%q", strings.TrimSpace(args))
	}
	e.Messages = nil
	e.Transcript = nil
	if err := e.SaveTranscript(); err != nil {
		slog.Error("session.transcript_save_reset", "err", err)
	}
	if err := e.SaveWorkingTranscript(); err != nil {
		slog.Error("session.working_transcript_save_reset", "err", err)
	}
	return strings.TrimSpace(`
已清空本会话的模型上下文与 slim 转录（内存与已配置的磁盘路径）。
磁盘上的 MEMORY.md 等说明文件未删除。本地斜杠命令的确认文案不会写入会话转录。
`)
}
