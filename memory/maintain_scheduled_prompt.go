package memory

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func buildScheduledToolUserPrompt(layout Layout, memPath string, p distillConfig, statePath, digestHeader, dateStr string, sameDayDigest bool) string {
	autoAbs := filepath.Clean(layout.Auto)
	projMemAbs := filepath.Clean(layout.Project)
	todayLog := filepath.Clean(DailyLogPath(layout.Auto, dateStr))

	var timeHint string
	lastWall, lineHW, _ := loadScheduledState(statePath)
	if p.incrementalInterval > 0 {
		minX := incrementalLineMinExclusive(lastWall, lineHW, p.incrementalInterval)
		timeHint = fmt.Sprintf(
			"**Incremental window:** prioritize daily log **lines** (they start with `- RFC3339 |`) whose timestamp is **after** %s UTC.\n"+
				"(This matches the last successful scheduled pass plus overlap, or ~one interval on first run.)\n\n",
			minX.Format(time.RFC3339),
		)
	} else {
		if lastWall != nil && !lastWall.IsZero() {
			b := lastWall.UTC().Add(-maintenanceIncrementalOverlap())
			timeHint = fmt.Sprintf(
				"**Calendar pass:** review daily logs under the auto-memory tree for roughly the **last %d calendar days**, "+
					"but give **priority** to lines with timestamps **after** %s UTC when possible.\n\n",
				p.logDays, b.Format(time.RFC3339),
			)
		} else {
			timeHint = fmt.Sprintf(
				"**Calendar pass:** review daily logs for roughly the **last %d calendar days** (see paths below).\n\n",
				p.logDays,
			)
		}
	}

	agentRoot := filepath.Join(layout.CWD, AgentInstructionsFile)
	agentDot := filepath.Join(layout.CWD, DotDir, AgentInstructionsFile)
	dialogDay := filepath.Clean(filepath.Join(layout.CWD, DotDir, "memory", dateStr, "dialog_history.json"))
	workingT := filepath.Clean(filepath.Join(layout.CWD, DotDir, "working_transcript.json"))
	slimT := filepath.Clean(filepath.Join(layout.CWD, DotDir, "transcript.json"))

	sameDayNote := ""
	if sameDayDigest {
		sameDayNote = "There is already a **" + digestHeader + "** section for today in MEMORY.md. Your output will be **merged** into it: use the **exact** same first line, then **only net-new or updated** durable bullets (paraphrases of existing same-day lines are redundant).\n\n"
	}

	return fmt.Sprintf(
		"%s%s"+
			"You are in **scheduled / far-field** memory maintenance. Use **read_file**, **grep**, **glob**, and **list_dir** "+
			"to inspect project memory and **auto daily logs**, then consolidate into MEMORY.md.\n\n"+
			"**Allowed tools only** (read-only): `read_file`, `grep`, `glob`, `list_dir`. "+
			"Do **not** call write_file, bash, run_agent, fork_context, cron, task_*, invoke_skill, or any mutating tool.\n\n"+
			"**Paths (absolute):**\n"+
			"- Auto memory root (daily logs live under `logs/YYYY/MM/YYYY-MM-DD.md`): `%s`\n"+
			"- Today's daily log file: `%s`\n"+
			"- Project MEMORY.md: `%s`\n"+
			"- Project topic markdown files (*.md except MEMORY.md): directory `%s`\n"+
			"- Behavior instructions (read if present; do **not** invent content you did not read): `%s` or `%s`\n"+
			"- Session dialog JSON for calendar day **%s** (slim user/assistant turns): `%s`\n"+
			"- Working model transcript, optional (tool rows + byte-budget compact recaps): `%s`\n"+
			"- Cumulative slim transcript, optional: `%s`\n\n"+
			"%s"+
			"**Task:** Read what you need via tools. Merge duplicates, update stale rules, and capture durable facts across sessions. "+
			"Be **terse**: one short sentence per bullet; **do not** paste long paths unless the path itself is the fact; **do not** claim a file exists unless you read it successfully.\n\n"+
			"**Final assistant message (markdown only):** first line must be **exactly**:\n%s\n\n"+
			"Then **3–8** bullet lines of **new** or **updated** durable facts, merged rules, or explicit notes that **supersede** stale MEMORY.md lines. "+
			"Skip pure duplicates of what is already correct in MEMORY.md or topics (paraphrases count). "+
			"If nothing needs changing, a single line: \"- (no durable entries)\". No preamble before the header; no extra sections after the bullets.",
		timeHint,
		sameDayNote,
		autoAbs,
		todayLog,
		filepath.Clean(memPath),
		projMemAbs,
		agentRoot,
		agentDot,
		dateStr,
		dialogDay,
		workingT,
		slimT,
		strings.TrimSpace(scheduledTopicHint(p.maxTopicFiles)),
		digestHeader,
	)
}

func scheduledTopicHint(maxFiles int) string {
	if maxFiles <= 0 {
		return ""
	}
	return fmt.Sprintf("There may be up to **%d** topic `*.md` files in the project memory directory; read only what you need.\n\n", maxFiles)
}
