package memory

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func buildScheduledToolUserPrompt(layout Layout, rulesMemPath, episodePath string, p distillConfig, statePath, digestHeader, dateStr string, sameDayDigest bool) string {
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

	agentDot := filepath.Join(layout.CWD, DotDir, AgentInstructionsFile)
	dialogDay := filepath.Clean(filepath.Join(layout.CWD, DotDir, "memory", dateStr, "dialog_history.json"))
	workingT := filepath.Clean(filepath.Join(layout.CWD, DotDir, "working_transcript.json"))
	slimT := filepath.Clean(filepath.Join(layout.CWD, DotDir, "transcript.json"))

	sameDayNote := ""
	if sameDayDigest {
		sameDayNote = "There is already a **" + digestHeader + "** section for today in the **episodic digest file** (see path below). Your output will be **merged** into it: use the **exact** same first line, then **only net-new or updated** durable bullets (paraphrases of existing same-day lines are redundant).\n\n"
	}

	return fmt.Sprintf(
		"%s%s"+
			"You are in **scheduled / far-field** memory maintenance. Use **read_file**, **grep**, **glob**, and **list_dir** "+
			"to inspect project memory and **auto daily logs**. Your **final assistant message** is merged into the **episodic digest** file for today (path below). **`MEMORY.md` holds project rules only** (loaded every turn like AGENT); put **durable episodic facts** in the digest, not in MEMORY.md.\n\n"+
			"**Allowed tools:** `read_file`, `grep`, `glob`, `list_dir`, and **`write_behavior_policy`** only when durable **instructions** need updating — "+
			"**`<cwd>/.oneclaw/AGENT.md`**, **`<cwd>/.oneclaw/rules/*.md`**, **`<cwd>/.oneclaw/skills/<name>/SKILL.md`**, **`MEMORY.md`** rules (target `memory`; full-file replace of rules only). "+
			"Do **not** call `write_file`, exec, run_agent, fork_context, cron, task_*, invoke_skill, or any other tool.\n\n"+
			"**Paths (absolute):**\n"+
			"- Auto memory root (daily logs live under `logs/YYYY/MM/YYYY-MM-DD.md`): `%s`\n"+
			"- Today's daily log file: `%s`\n"+
			"- Project **rules** (`MEMORY.md`): `%s`\n"+
			"- Today's **episodic digest** (your output is merged here): `%s`\n"+
			"- Project topic markdown files (*.md except MEMORY.md): directory `%s`\n"+
			"- Project agent instructions (read if present; do **not** invent content you did not read): `%s`\n"+
			"- Session dialog JSON for calendar day **%s** (slim user/assistant turns): `%s`\n"+
			"- Working model transcript, optional (tool rows + byte-budget compact recaps): `%s`\n"+
			"- Cumulative slim transcript, optional: `%s`\n\n"+
			"%s"+
			"**Task:** Read what you need via tools. Merge duplicates and capture durable **episodic** facts into the digest file. "+
			"For **standing rules** (how the agent should behave), prefer **`.oneclaw/AGENT.md`**, **rules**, **skills**, or **`MEMORY.md`** via `write_behavior_policy` — keep MEMORY.md compact. "+
			"Be **terse**: one short sentence per bullet; **do not** paste long paths unless the path itself is the fact; **do not** claim a file exists unless you read it successfully.\n\n"+
			"**Final assistant message (markdown only):** first line must be **exactly**:\n%s\n\n"+
			"Then **3–8** bullet lines of **new** or **updated** durable **episodic** facts (or explicit notes that obsolete prior digest bullets). "+
			"Skip pure duplicates of what is already correct in the rules file, today's digest, or topics (paraphrases count). "+
			"If nothing needs changing, a single line: \"- (no durable entries)\". No preamble before the header; no extra sections after the bullets.",
		timeHint,
		sameDayNote,
		autoAbs,
		todayLog,
		filepath.Clean(rulesMemPath),
		filepath.Clean(episodePath),
		projMemAbs,
		filepath.Clean(agentDot),
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
