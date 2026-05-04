package wfexec

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/session"
	"github.com/lengzhao/oneclaw/workflow"
)

// BuildSubagentUserPrompt assembles the initial user message for use: agent from params.context (and legacy defaults).
func BuildSubagentUserPrompt(rtx *engine.RuntimeContext) (string, error) {
	if rtx == nil {
		return "", fmt.Errorf("wfexec: nil runtime context")
	}
	params := rtx.CurrentParams
	if len(params) == 0 {
		params = map[string]any{}
	}
	atts := workflow.ParseAgentContextAttachments(params)
	if len(atts) == 0 {
		return defaultTurnPairPrompt(rtx), nil
	}
	host := hostCatalogAgentID(rtx)
	var b strings.Builder
	for _, a := range atts {
		block, err := materializeAttachment(rtx, host, a)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(block) == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(block)
	}
	if b.Len() == 0 {
		return defaultTurnPairPrompt(rtx), nil
	}
	return b.String(), nil
}

func hostCatalogAgentID(rtx *engine.RuntimeContext) string {
	s := strings.TrimSpace(rtx.Turn.AgentID)
	if s == "" && rtx.Agent != nil {
		s = rtx.Agent.AgentType
	}
	return s
}

func defaultTurnPairPrompt(rtx *engine.RuntimeContext) string {
	var b strings.Builder
	b.WriteString("Context for this agent run:\n\nUser message:\n")
	b.WriteString(strings.TrimSpace(rtx.EffectiveUserPrompt()))
	if a := strings.TrimSpace(rtx.Assistant); a != "" {
		b.WriteString("\n\nMain agent assistant reply (extract durable facts from both sides; quote assistant wording when it states identity, names, or commitments):\n")
		b.WriteString(a)
	}
	return b.String()
}

func materializeAttachment(rtx *engine.RuntimeContext, host string, a workflow.AgentContextAttachment) (string, error) {
	label := strings.TrimSpace(a.Label)
	header := "## Context attachment\n\n"
	if label != "" {
		header = "## " + label + "\n\n"
	}
	switch a.Ref {
	case workflow.AgentContextRefRunJournal:
		return materializeRunJournal(rtx, host, header, a)
	case workflow.AgentContextRefWorkflowNode:
		return materializeWorkflowNode(rtx, header, a)
	case workflow.AgentContextRefTranscript:
		return materializeTranscript(rtx, header)
	default:
		return "", fmt.Errorf("wfexec: unknown context ref %q", a.Ref)
	}
}

func materializeRunJournal(rtx *engine.RuntimeContext, host, header string, a workflow.AgentContextAttachment) (string, error) {
	scope := strings.TrimSpace(a.Scope)
	if scope == "" {
		scope = workflow.AgentContextScopeCurrentTurn
	}
	as := strings.TrimSpace(a.As)
	if as == "" {
		as = workflow.AgentContextAsUserMessage
	}
	sr := strings.TrimSpace(rtx.EffectiveSessionRoot())
	if sr == "" || host == "" {
		return "", fmt.Errorf("wfexec: run_journal context needs session root and host agent id")
	}
	path := filepath.Join(sr, "runs", host, "runs.jsonl")

	switch as {
	case workflow.AgentContextAsToolBinding:
		var tip strings.Builder
		tip.WriteString(header)
		tip.WriteString("Use the read_run_journal tool to load the execution journal before proceeding.")
		corr := strings.TrimSpace(rtx.CorrelationID)
		if corr != "" {
			tip.WriteString(` Use scope "current_turn".`)
		} else {
			tip.WriteString(` Use scope "full" if correlation_id is unavailable.`)
		}
		tip.WriteString("\n\nJournal path (reference): ")
		tip.WriteString(path)
		tip.WriteByte('\n')
		return tip.String(), nil
	case workflow.AgentContextAsUserMessage:
		text, err := session.ReadRunJournalText(sr, host, strings.TrimSpace(rtx.CorrelationID), scope)
		if err != nil {
			return "", fmt.Errorf("wfexec: run_journal: %w", err)
		}
		return header + "```jsonl\n" + text + "\n```", nil
	case workflow.AgentContextAsPathMetadata:
		var meta strings.Builder
		meta.WriteString(header)
		meta.WriteString("run_journal_path: ")
		meta.WriteString(path)
		meta.WriteByte('\n')
		info, err := os.Stat(path)
		if err != nil {
			meta.WriteString("size_bytes: unknown\n")
			meta.WriteString("stat_note: ")
			meta.WriteString(err.Error())
			meta.WriteByte('\n')
		} else {
			meta.WriteString(fmt.Sprintf("size_bytes: %d\n", info.Size()))
		}
		meta.WriteString("workflow_scope_hint: ")
		meta.WriteString(scope)
		meta.WriteString("\n")
		return meta.String(), nil
	default:
		return "", fmt.Errorf("wfexec: run_journal: unsupported as %q", as)
	}
}

func materializeWorkflowNode(rtx *engine.RuntimeContext, header string, a workflow.AgentContextAttachment) (string, error) {
	sel := a.Select
	if len(sel) == 0 {
		return "", fmt.Errorf("wfexec: workflow_node context requires select")
	}
	nodeID := strings.TrimSpace(fmt.Sprint(sel["node_id"]))
	if nodeID == "" {
		return "", fmt.Errorf("wfexec: workflow_node select.node_id required")
	}
	if rtx.WorkflowNodeOutputs == nil {
		return "", fmt.Errorf("wfexec: no workflow node outputs for %q", nodeID)
	}
	payload := rtx.WorkflowNodeOutputs[nodeID]
	if len(payload) == 0 {
		return "", fmt.Errorf("wfexec: empty workflow output for node %q", nodeID)
	}

	fieldsRaw := sel["fields"]
	var fields []string
	switch fr := fieldsRaw.(type) {
	case []any:
		for _, x := range fr {
			s := strings.TrimSpace(fmt.Sprint(x))
			if s != "" {
				fields = append(fields, s)
			}
		}
	case nil:
	default:
		s := strings.TrimSpace(fmt.Sprint(fr))
		if s != "" {
			fields = append(fields, s)
		}
	}

	out := make(map[string]any)
	if len(fields) == 0 {
		for k, v := range payload {
			out[k] = v
		}
	} else {
		for _, k := range fields {
			if v, ok := payload[k]; ok {
				out[k] = v
			}
		}
	}
	js, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	return header + string(js), nil
}

func materializeTranscript(rtx *engine.RuntimeContext, header string) (string, error) {
	sr := strings.TrimSpace(rtx.EffectiveSessionRoot())
	if sr == "" {
		return "", fmt.Errorf("wfexec: transcript context needs session root")
	}
	turns, err := session.LoadTranscriptTurns(sr)
	if err != nil {
		return "", err
	}
	turns = session.TrimTranscriptTail(turns, session.DefaultTranscriptTurnLimit)
	var b strings.Builder
	b.WriteString(header)
	for _, t := range turns {
		role := strings.TrimSpace(t.Role)
		content := strings.TrimSpace(t.Content)
		if role == "" || content == "" {
			continue
		}
		b.WriteString(role)
		b.WriteString(": ")
		b.WriteString(content)
		b.WriteByte('\n')
	}
	return b.String(), nil
}
