package main

import (
	"fmt"
	"io"
)

func writeUsage(w io.Writer, prog string) {
	fmt.Fprintf(w, `Usage: %s [global-options] <command> [args...]

Global options (FR-CFG-04):
  -config PATH       Config file (optional; run loads <UserDataRoot>/config.yaml when omitted)
  -log-level LEVEL   debug|info|warn|error (default: info)
  -log-format FMT    text|json (default: text)

Commands:
  init       Bootstrap UserDataRoot (flags: --user-data; merges config keys if config.yaml exists)
  run, repl  Single-turn agent (flags: --mock-llm, --profile, --agent, --prompt, --session)
  serve      clawbridge + WebChat + TurnHub + optional schedule (flags: --no-schedule, --mock-llm); send /reset to clear transcript.jsonl only (runs/subs unchanged)
  channel    clawbridge driver onboarding: list-drivers | onboard <driver> (see -h)
  snapshot   Export session snapshot for backup/migration (stub)
  version    Print version
  help       Show this message

`, prog)
}
