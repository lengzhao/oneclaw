package schedule

import (
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/budget"
)

const maxSystemJobs = 32
const maxSystemBytes = 8000

// PromptLines returns file path and one markdown line per enabled job (for system prompt).
func PromptLines(cwd, hostDataRoot string) (filePath string, lines []string, omitted int) {
	if Disabled() {
		return "", nil, 0
	}
	path := JobsFilePath(cwd, hostDataRoot)
	f, err := Read(path)
	if err != nil || len(f.Jobs) == 0 {
		return "", nil, 0
	}
	var enabled []Job
	for _, j := range f.Jobs {
		if j.Enabled {
			enabled = append(enabled, j)
		}
	}
	if len(enabled) == 0 {
		return path, nil, 0
	}
	n := len(enabled)
	if n > maxSystemJobs {
		omitted = n - maxSystemJobs
		n = maxSystemJobs
	}
	lines = make([]string, 0, n)
	for i := 0; i < n; i++ {
		j := enabled[i]
		sch := formatSchedule(j.Schedule)
		next := "pending"
		if !j.NextRun.IsZero() {
			next = j.NextRun.UTC().Format("2006-01-02 15:04 MST")
		}
		line := "- `" + j.ID + "` **" + j.Name + "** — " + sch + " → next " + next + " — " + strings.TrimSpace(j.Message)
		lines = append(lines, line)
	}
	return path, lines, omitted
}

// SystemBlock returns a markdown section for the model (empty if disabled or no jobs).
func SystemBlock(cwd, hostDataRoot string) string {
	path, lines, omitted := PromptLines(cwd, hostDataRoot)
	if len(lines) == 0 && path == "" {
		return ""
	}
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Scheduled jobs (persisted)\n\n")
	b.WriteString("Timed prompts are stored at `" + path + "`. Use the **cron** tool to add, list, or remove entries (remove stops a recurring job).\n\n")
	for _, ln := range lines {
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "\n… and %d more (see file or `cron` list)\n", omitted)
	}
	return budget.TruncateUTF8(b.String(), maxSystemBytes)
}
