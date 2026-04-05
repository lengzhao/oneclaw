package memory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TurnBundle is injected into one user turn (system suffix + leading user messages).
type TurnBundle struct {
	SystemSuffix  string
	AgentMdBlock  string // AGENT.md + rules context in <system-reminder> user message
	RecallBlock   string // user message with attachment header; empty if none
	UpdatedRecall *RecallState
}

// BuildTurn assembles discovery, system memory indices, and recall for this turn.
// recallBudget caps SelectRecall output (bytes); if <= 0, MaxSurfacedRecallBytes is used.
func BuildTurn(layout Layout, home, userText string, recall *RecallState, recallBudget int) TurnBundle {
	layout.EnsureDirs()

	var sys strings.Builder
	sys.WriteString("\n## File-based memory\n\n")
	sys.WriteString("You have persistent, file-based memory. These directories already exist — write with the Write tool (no need to mkdir first).\n\n")
	writeDirList(&sys, layout)

	type ep struct {
		label string
		path  string
	}
	eps := []ep{
		{"user", filepath.Join(layout.User, entrypointName)},
		{"project", filepath.Join(layout.Project, entrypointName)},
		{"local", filepath.Join(layout.Local, entrypointName)},
		{"team (user)", filepath.Join(layout.TeamUser, entrypointName)},
		{"team (project)", filepath.Join(layout.TeamProject, entrypointName)},
	}
	if !AutoMemoryDisabled() {
		eps = append(eps, ep{"auto", filepath.Join(layout.Auto, entrypointName)})
	}
	labels := []string{"user-scope agent", "project-scope agent", "local-scope agent"}
	for i, root := range layout.AgentDefault {
		label := "agent"
		if i < len(labels) {
			label = labels[i]
		}
		eps = append(eps, ep{label, filepath.Join(root, entrypointName)})
	}

	sys.WriteString("### MEMORY.md indices (truncated when large)\n\n")
	for _, e := range eps {
		if chunk := readTruncatedEntrypoint(e.path); chunk != "" {
			sys.WriteString(fmt.Sprintf("#### %s (`%s`)\n\n%s\n\n", e.label, e.path, chunk))
		}
	}

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
	for _, chunk := range DiscoverProjectInstructions(layout.CWD) {
		body := LoadMarkdownBody(chunk.Path)
		ctx.WriteString(fmt.Sprintf("### %s:%s\n\n%s\n\n", chunk.Kind, chunk.Path, body))
	}

	ctx.WriteString("# currentDate\n\n")
	ctx.WriteString(fmt.Sprintf("Today's date is %s.\n", time.Now().Format("2006-01-02")))
	ctx.WriteString("\n</system-reminder>")

	agentMd := strings.TrimSpace(ctx.String())
	if recallBudget <= 0 {
		recallBudget = MaxSurfacedRecallBytes
	}
	recallBody, nextRecall := SelectRecall(layout, userText, recall, recallBudget)

	return TurnBundle{
		SystemSuffix:  sys.String(),
		AgentMdBlock:  agentMd,
		RecallBlock:   recallBody,
		UpdatedRecall: nextRecall,
	}
}

func writeDirList(sb *strings.Builder, layout Layout) {
	fmt.Fprintf(sb, "- **user** — `%s`\n", layout.User)
	fmt.Fprintf(sb, "- **project** — `%s`\n", layout.Project)
	fmt.Fprintf(sb, "- **local** — `%s`\n", layout.Local)
	if !AutoMemoryDisabled() {
		fmt.Fprintf(sb, "- **auto** — `%s`\n", layout.Auto)
	}
	fmt.Fprintf(sb, "- **team (user)** — `%s`\n", layout.TeamUser)
	fmt.Fprintf(sb, "- **team (project)** — `%s`\n", layout.TeamProject)
	for i, root := range layout.AgentDefault {
		fmt.Fprintf(sb, "- **agent memory (%d)** — `%s`\n", i, root)
	}
	sb.WriteString("\n")
}

func readTruncatedEntrypoint(abs string) string {
	b, err := os.ReadFile(abs)
	if err != nil {
		return ""
	}
	return TruncateEntrypointContent(string(b)).Content
}
