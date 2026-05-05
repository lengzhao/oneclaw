package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/lengzhao/oneclaw/session"
)

// NameReadRunJournal reads per-turn JSONL under sessions/<id>/runs/<agent>/ (merged when scope is full).
const NameReadRunJournal = "read_run_journal"

type readRunJournalIn struct {
	AgentType string `json:"agent_type,omitempty" jsonschema:"description=Catalog agent id segment under runs/; omit to use the host turn agent"`
	Scope     string `json:"scope,omitempty" jsonschema:"description=current_turn: JSONL lines whose detail.correlation_id matches this turn (default). full: entire file."`
}

// InferReadRunJournal builds read_run_journal bound to the host session and correlation id.
func InferReadRunJournal(sessionRoot, hostAgentID, correlationID string) (tool.InvokableTool, error) {
	sr := strings.TrimSpace(sessionRoot)
	if sr == "" {
		return nil, fmt.Errorf("%s: session root required", NameReadRunJournal)
	}
	return utils.InferTool(NameReadRunJournal,
		`Read UTF-8 JSONL run records under sessions/.../runs/<agent_type>/: per-turn files <correlation_id>.jsonl (and sub-agent keys parentcorr__subrun.jsonl). scope current_turn (default) reads that single turn file (retry until complete). scope full merges all per-turn *.jsonl in the directory (excluding legacy runs.jsonl). correlation_id empty + either scope merges all journals.`,
		func(ctx context.Context, in readRunJournalIn) (string, error) {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			at := strings.TrimSpace(in.AgentType)
			if at == "" {
				at = strings.TrimSpace(hostAgentID)
			}
			if at == "" {
				return "", fmt.Errorf("agent_type required (host agent id missing)")
			}
			scope := strings.TrimSpace(in.Scope)
			if scope == "" {
				scope = "current_turn"
			}
			return session.ReadRunJournalText(sr, at, correlationID, scope)
		})
}
