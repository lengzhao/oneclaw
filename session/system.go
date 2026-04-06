package session

import (
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/prompts"
	"github.com/lengzhao/oneclaw/schedule"
	"github.com/lengzhao/oneclaw/skills"
	"github.com/lengzhao/oneclaw/tasks"
)

// MainThreadSystemData drives prompts/templates/main_thread_system.tmpl (see docs/prompts/10-main-thread.md).
// Slices that are nil or empty omit their {{range}} / {{if}} blocks in the template.
type MainThreadSystemData struct {
	CWD           string
	Platform      string
	Shell         string
	TasksFilePath string
	TaskLines     []string
	TasksOmitted  int

	MemoryPromptBlock string

	SkillLines []string

	OptionalMCPSection string

	OptionalScratchpadSection string

	AppendedSystemContext string
}

func shellForPrompt() string {
	s := strings.TrimSpace(os.Getenv("SHELL"))
	if s == "" {
		return "(unknown)"
	}
	return s
}

// buildTurnSystem renders the main-thread system prompt (docs/prompts/10-main-thread.md).
// e.System is appended last as "Additional system context" only; the default identity lives in prompts/templates/main_thread_system.tmpl.
func (e *Engine) buildTurnSystem(memOK bool, bundle memory.TurnBundle, bg budget.Global, home string, herr error) string {
	d := MainThreadSystemData{
		CWD:                   e.CWD,
		Platform:              runtime.GOOS,
		Shell:                 shellForPrompt(),
		AppendedSystemContext: strings.TrimSpace(e.System),
	}
	if memOK {
		d.MemoryPromptBlock = strings.TrimSpace(bundle.SystemSuffix)
	}
	p, lines, omit := tasks.PromptTaskLines(e.CWD)
	d.TasksFilePath = p
	d.TaskLines = lines
	d.TasksOmitted = omit
	if herr == nil {
		d.SkillLines = skills.PromptSkillLines(e.CWD, home, bg.SkillIndexMaxBytes())
	}

	out, err := prompts.Render(prompts.NameMainThreadSystem, d)
	if err != nil {
		slog.Error("session.main_thread_system", "err", err)
		return fallbackMainThreadSystem(d)
	}
	out = strings.TrimRight(out, "\n")
	if sb := schedule.SystemBlock(e.CWD); sb != "" {
		out += sb + "\n"
	}
	return out + "\n"
}

func fallbackMainThreadSystem(d MainThreadSystemData) string {
	var b strings.Builder
	b.WriteString("You are Oneclaw, a coding agent. Be concise. Use tools when they help answer accurately.\n")
	b.WriteString("Working directory for file tools is ")
	b.WriteString(d.CWD)
	b.WriteString(". Prefer read_file before editing.\n")
	if d.MemoryPromptBlock != "" {
		b.WriteString("\n")
		b.WriteString(d.MemoryPromptBlock)
		b.WriteString("\n")
	}
	if len(d.TaskLines) > 0 && d.TasksFilePath != "" {
		b.WriteString("\n## Task list (persisted)\n\n")
		b.WriteString("Structured work items are stored at `" + d.TasksFilePath + "`. Use `task_create` / `task_update` to keep them accurate across turns and restarts.\n\n")
		for _, ln := range d.TaskLines {
			b.WriteString(ln)
			b.WriteByte('\n')
		}
		if d.TasksOmitted > 0 {
			fmt.Fprintf(&b, "\n… and %d more (not shown; see file or use tools)\n", d.TasksOmitted)
		}
	}
	if len(d.SkillLines) > 0 {
		b.WriteString("\n## Skills\n\n")
		b.WriteString("When a task matches a skill, call **invoke_skill** with that skill's name to load its full instructions (body of SKILL.md).\n\n")
		b.WriteString(strings.Join(d.SkillLines, "\n"))
		b.WriteString("\n")
	}
	if d.AppendedSystemContext != "" {
		b.WriteString("\n")
		b.WriteString(d.AppendedSystemContext)
		b.WriteString("\n")
	}
	out := strings.TrimRight(b.String(), "\n")
	if sb := schedule.SystemBlock(d.CWD); sb != "" {
		out += sb + "\n"
	}
	return out + "\n"
}
