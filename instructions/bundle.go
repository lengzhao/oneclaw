package instructions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/workspace"
)

// TurnBundle is injected into one user turn (system suffix + leading user messages).
type TurnBundle struct {
	SystemSuffix string
	AgentMdBlock string // AGENT.md + rules + MEMORY.md / SOUL.md / TODO.md in <system-reminder> user message
}

// BuildTurn assembles discovery and instruction files for this turn (no recall / episodic search).
func BuildTurn(layout workspace.Layout, home string) TurnBundle {
	layout.EnsureDirs()

	var sys strings.Builder
	sys.WriteString("\n## Persistent instruction files\n\n")
	sys.WriteString("Standing rules and notes live in **`AGENT.md`**, **`MEMORY.md`**, optional **`SOUL.md`** / **`TODO.md`** (same directory as `AGENT.md` in IM layouts), and under **`~/.oneclaw/`** for user-wide defaults. ")
	sys.WriteString("Structured tasks use **`tasks.json`** (see the Task list section in the system prompt). ")
	sys.WriteString("Use the Write tool only under allowed paths; `write_behavior_policy` updates rules, skills, and canonical instruction files.\n\n")

	var ctx strings.Builder
	ctx.WriteString("Codebase and user instructions are shown below. Follow them; they override defaults.\n\n")
	ctx.WriteString("<system-reminder>\n# agentMd\n\n")

	for _, chunk := range DiscoverUserAgentMd(home) {
		body := LoadMarkdownBody(chunk.Path)
		ctx.WriteString(fmt.Sprintf("### user:%s\n\n%s\n\n", chunk.Path, body))
	}
	for _, chunk := range DiscoverUserRules(home) {
		body := LoadMarkdownBody(chunk.Path)
		ctx.WriteString(fmt.Sprintf("### user rules:%s\n\n%s\n\n", chunk.Path, body))
	}
	appendEntrypointFile(&ctx, "user:memory", filepath.Join(layout.User, workspace.RulesMemoryFile))
	if !layout.HostUserData {
		for _, chunk := range DiscoverProjectInstructions(layout.CWD) {
			body := LoadMarkdownBody(chunk.Path)
			ctx.WriteString(fmt.Sprintf("### %s:%s\n\n%s\n\n", chunk.Kind, chunk.Path, body))
		}
	} else if layout.InstructionRoot != "" {
		for _, chunk := range DiscoverFlatDotRootInstructions(layout.InstructionRoot) {
			body := LoadMarkdownBody(chunk.Path)
			ctx.WriteString(fmt.Sprintf("### %s:%s\n\n%s\n\n", chunk.Kind, chunk.Path, body))
		}
	} else if filepath.Clean(layout.CWD) != filepath.Clean(layout.MemoryBase) {
		for _, chunk := range DiscoverFlatDotRootInstructions(layout.CWD) {
			body := LoadMarkdownBody(chunk.Path)
			ctx.WriteString(fmt.Sprintf("### %s:%s\n\n%s\n\n", chunk.Kind, chunk.Path, body))
		}
	}
	dir := layout.RulesEntryDir()
	rulesAbs := filepath.Join(dir, workspace.RulesMemoryFile)
	appendEntrypointFile(&ctx, "project:memory", rulesAbs)
	appendEntrypointFile(&ctx, "project:soul", filepath.Join(dir, workspace.SoulFile))
	appendEntrypointFile(&ctx, "project:todo", filepath.Join(dir, workspace.TodoFile))
	appendEntrypointFile(&ctx, "user:soul", filepath.Join(layout.User, workspace.SoulFile))
	appendEntrypointFile(&ctx, "user:todo", filepath.Join(layout.User, workspace.TodoFile))

	ctx.WriteString("# currentDate\n\n")
	ctx.WriteString(fmt.Sprintf("Today's date is %s.\n", time.Now().Format("2006-01-02")))
	ctx.WriteString("\n</system-reminder>")

	agentMd := strings.TrimSpace(ctx.String())

	return TurnBundle{
		SystemSuffix: sys.String(),
		AgentMdBlock: agentMd,
	}
}

func appendEntrypointFile(sb *strings.Builder, label, abs string) {
	chunk := readTruncatedEntrypoint(abs)
	if chunk == "" {
		return
	}
	sb.WriteString(fmt.Sprintf("### %s (`%s`)\n\n%s\n\n", label, abs, chunk))
}

func readTruncatedEntrypoint(abs string) string {
	b, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	name := filepath.Base(abs)
	return TruncateEntrypointContent(string(b), name).Content
}
