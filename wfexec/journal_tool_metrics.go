package wfexec

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/journaltools"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/workflow"
)

func handleJournalToolMetrics(_ context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	rtx := env.Runtime
	if rtx == nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: journal_tool_metrics: nil runtime")
	}
	jp, udr, ok := resolveJournalPathForMetrics(strings.TrimSpace(in.Text), rtx)
	if !ok {
		return workflowNodeResultMetrics(journaltools.Metrics{}, "")
	}
	opts := &journaltools.ScanOpts{MatchTools: matchToolsFromParams(env.Node.Params)}
	m, err := journaltools.ScanJournalFile(jp, udr, opts)
	if err != nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: journal_tool_metrics: %w", err)
	}
	return workflowNodeResultMetrics(m, jp)
}

func resolveJournalPathForMetrics(inText string, rtx *engine.RuntimeContext) (journalPath string, userDataRoot string, ok bool) {
	if rtx == nil {
		return "", "", false
	}
	udr := strings.TrimSpace(rtx.EffectiveUserDataRoot())
	inText = strings.TrimSpace(inText)
	if inText != "" {
		if p, parsed := parsePostTurnPayloadYAML(inText); parsed && strings.TrimSpace(p.RunJournal.Path) != "" {
			r := mergePayloadWithRuntime(p, rtx)
			if validateRunJournalPath(r.JournalPath, r.SessionRoot, r.HostAgentID) == nil {
				if ud := strings.TrimSpace(p.UserDataRoot); ud != "" {
					udr = ud
				}
				return r.JournalPath, udr, true
			}
		}
		if inf, err := inferStructuredExtractFromJournalPath(inText, rtx); err == nil {
			if validateRunJournalPath(inf.JournalPath, inf.SessionRoot, inf.HostAgentID) == nil {
				return inf.JournalPath, udr, true
			}
		}
	}
	// Sub-agents under subs/<id>/ default to runtimeAgentType=sub workflow agent (e.g. skill_generator),
	// which points journal_tool_metrics at the wrong JSONL. Prefer the delegating host session when present.
	psr := strings.TrimSpace(rtx.ParentSessionRoot)
	phost := paths.SanitizeSessionPathSegment(strings.TrimSpace(rtx.ParentAgentType))
	corr := strings.TrimSpace(rtx.CorrelationID)
	if psr != "" && phost != "" && corr != "" {
		jp := session.TurnRunJournalPath(psr, phost, corr)
		if validateRunJournalPath(jp, psr, phost) == nil {
			return jp, udr, true
		}
	}
	sr := strings.TrimSpace(rtx.EffectiveSessionRoot())
	host := runtimeAgentType(rtx)
	if sr == "" || host == "" || corr == "" {
		return "", udr, false
	}
	return session.TurnRunJournalPath(sr, host, corr), udr, true
}

func matchToolsFromParams(params map[string]any) []string {
	if len(params) == 0 {
		return nil
	}
	raw, ok := params["match_tools"]
	if !ok || raw == nil {
		return nil
	}
	switch x := raw.(type) {
	case []string:
		return append([]string(nil), x...)
	case []any:
		out := make([]string, 0, len(x))
		for _, el := range x {
			s := strings.TrimSpace(fmt.Sprint(el))
			if s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

func workflowNodeResultMetrics(m journaltools.Metrics, journalPath string) (workflow.WorkflowNodeResult, error) {
	b, err := json.Marshal(m)
	if err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	data := map[string]any{
		"distinct_tool_calls":  m.DistinctToolCalls,
		"skill_tree_tool_used": m.SkillTreeToolUsed,
		"tool_call_events":     m.ToolCallEvents,
		"line_count":           m.LineCount,
		"duration_seconds":     m.DurationSeconds,
		"has_run_start":        m.HasRunStart,
		"has_run_complete":     m.HasRunComplete,
	}
	if strings.TrimSpace(m.FirstTs) != "" {
		data["first_ts"] = m.FirstTs
	}
	if strings.TrimSpace(m.LastTs) != "" {
		data["last_ts"] = m.LastTs
	}
	if strings.TrimSpace(journalPath) != "" {
		data["journal_path"] = journalPath
	}
	for name, present := range m.NamedToolsPresent {
		data[toolMatchedDataKey(name)] = present
	}
	for name, n := range m.NamedToolsCounts {
		data[toolMatchedCountDataKey(name)] = n
	}
	return workflow.WorkflowNodeResult{Text: string(b), Data: data}, nil
}

func toolMatchedDataKey(toolName string) string {
	return "tool_matched_" + safeMetricSuffix(toolName)
}

func toolMatchedCountDataKey(toolName string) string {
	return "tool_match_count_" + safeMetricSuffix(toolName)
}

func safeMetricSuffix(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		return "unknown"
	}
	return out
}
