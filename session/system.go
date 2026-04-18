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
	"github.com/lengzhao/oneclaw/subagent"
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

	AgentCatalogLines []string

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
// cat lists available run_agent roles (see docs/orchestrator-business-agents.md).
func (e *Engine) buildTurnSystem(memOK bool, bundle memory.TurnBundle, bg budget.Global, home string, herr error, cat *subagent.Catalog) string {
	d := MainThreadSystemData{
		CWD:                   e.CWD,
		Platform:              runtime.GOOS,
		Shell:                 shellForPrompt(),
		AppendedSystemContext: strings.TrimSpace(e.System),
	}
	if memOK {
		d.MemoryPromptBlock = strings.TrimSpace(bundle.SystemSuffix)
	}
	p, lines, omit := tasks.PromptTaskLines(e.CWD, e.WorkspaceFlat, e.InstructionRoot)
	d.TasksFilePath = p
	d.TaskLines = lines
	d.TasksOmitted = omit
	if herr == nil {
		d.SkillLines = skills.PromptSkillLines(e.CWD, home, bg.SkillIndexMaxBytes(), e.WorkspaceFlat, e.InstructionRoot)
	}
	if cat != nil {
		d.AgentCatalogLines = cat.PromptCatalogLines(bg.SkillIndexMaxBytes())
	}
	if s := strings.TrimSpace(e.MCPSystemNote); s != "" {
		d.OptionalMCPSection = s
	}

	out, err := prompts.Render(prompts.NameMainThreadSystem, d)
	if err != nil {
		slog.Error("session.main_thread_system", "err", err)
		return fallbackMainThreadSystem(d)
	}
	out = strings.TrimRight(out, "\n")
	if sb := schedule.SystemBlock(e.CWD, e.UserDataRoot, e.WorkspaceFlat, e.InstructionRoot); sb != "" {
		out += sb + "\n"
	}
	return out + "\n"
}

func fallbackMainThreadSystem(d MainThreadSystemData) string {
	var b strings.Builder
	b.WriteString("You are Oneclaw, a coding agent. Be concise. Use tools when they help answer accurately.\n")
	b.WriteString("Project and user rules appear in user messages inside <system-reminder> under # agentMd; when they conflict with these defaults, follow the agentMd / rules content.\n")
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
	if len(d.AgentCatalogLines) > 0 {
		b.WriteString("\n## Delegated agents (run_agent)\n\n")
		b.WriteString("Use **run_agent** with `agent_type` from the list below. Prefer delegating domain work; keep the main thread for routing, clarification, and merging results. Each `prompt` should be self-contained (goal, constraints, done criteria). Use `inherit_context` when the sub-agent needs a trimmed slice of the parent conversation.\n\n")
		b.WriteString(strings.Join(d.AgentCatalogLines, "\n"))
		b.WriteString("\n")
	}
	if d.AppendedSystemContext != "" {
		b.WriteString("\n")
		b.WriteString(d.AppendedSystemContext)
		b.WriteString("\n")
	}
	out := strings.TrimRight(b.String(), "\n")
	if sb := schedule.SystemBlock(d.CWD, "", false, ""); sb != "" {
		out += sb + "\n"
	}
	return out + "\n"
}
