package builtin

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"

	"github.com/lengzhao/oneclaw/session"
)

// NameReadRunJournal reads sessions/<id>/runs/<agent>/runs.jsonl for post-turn agents.
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
		`Read UTF-8 JSONL execution records for the host agent under sessions/.../runs/<agent_type>/runs.jsonl. scope current_turn (default) returns lines whose detail.correlation_id matches this invocation; use full for the entire journal.`,
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
