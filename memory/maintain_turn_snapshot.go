package memory

import (
	"fmt"
	"sort"
	"strings"

	"github.com/lengzhao/oneclaw/loop"
	"github.com/lengzhao/oneclaw/rtopts"
)

// formatMaintainTurnSnapshot builds a bounded text block for the post-turn maintenance user prompt.
func formatMaintainTurnSnapshot(in *PostTurnInput) string {
	if in == nil {
		return ""
	}
	rt := rtopts.Current()
	maxU := rt.PostTurnUserSnapshotBytes
	maxA := rt.PostTurnAssistantSnapshotBytes
	if maxU <= 0 {
		maxU = 4000
	}
	if maxA <= 0 {
		maxA = 8000
	}
	if maxU < 200 {
		maxU = 200
	}
	if maxA < 200 {
		maxA = 200
	}

	u := strings.TrimSpace(in.UserText)
	if len(u) > maxU {
		u = strings.TrimRight(utf8SafePrefix(u, maxU), "\n") + "\n…"
	}
	a := strings.TrimSpace(in.AssistantVisible)
	if len(a) > maxA {
		a = strings.TrimRight(utf8SafePrefix(a, maxA), "\n") + "\n…"
	}

	var b strings.Builder
	b.WriteString("session_id: ")
	b.WriteString(strings.TrimSpace(in.SessionID))
	b.WriteString("\ncorrelation_id: ")
	b.WriteString(strings.TrimSpace(in.CorrelationID))
	b.WriteString("\n\nuser:\n")
	b.WriteString(u)
	b.WriteString("\n\nassistant (visible):\n")
	b.WriteString(a)
	if td := formatMaintainToolDetail(in.Tools); td != "" {
		b.WriteString("\n\ntools (this turn):\n")
		b.WriteString(td)
	}
	return b.String()
}

// formatMaintainToolDetail lists each tool invocation for the maintenance snapshot and highlights repeats.
func formatMaintainToolDetail(entries []loop.ToolTraceEntry) string {
	if len(entries) == 0 {
		return ""
	}
	counts := make(map[string]int)
	for _, e := range entries {
		n := strings.TrimSpace(e.Name)
		if n != "" {
			counts[n]++
		}
	}
	var rep []string
	for name, n := range counts {
		if n > 1 {
			rep = append(rep, fmt.Sprintf("%s×%d", name, n))
		}
	}
	sort.Strings(rep)

	var b strings.Builder
	for _, e := range entries {
		b.WriteString("- step ")
		b.WriteString(fmt.Sprintf("%d", e.Step))
		b.WriteString(" ")
		b.WriteString(strings.TrimSpace(e.Name))
		if e.OK {
			b.WriteString(" ok")
		} else {
			b.WriteString(" err")
			if strings.TrimSpace(e.Err) != "" {
				b.WriteString("(")
				b.WriteString(oneLine(e.Err, 200))
				b.WriteString(")")
			}
		}
		if strings.TrimSpace(e.ArgsPreview) != "" {
			b.WriteString(" args=")
			b.WriteString(oneLine(e.ArgsPreview, 100))
		}
		if strings.TrimSpace(e.OutPreview) != "" {
			b.WriteString(" out=")
			b.WriteString(oneLine(e.OutPreview, 200))
		}
		b.WriteByte('\n')
	}
	if len(rep) > 0 {
		b.WriteString("repeated_in_this_turn: ")
		b.WriteString(strings.Join(rep, ", "))
		b.WriteString(" (infer why only from user/assistant text above, not from other sessions)\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
