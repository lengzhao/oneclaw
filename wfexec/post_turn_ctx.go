package wfexec

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/session"
)

// postTurnCtxDoc is the YAML envelope for $runtime.post_turn.ctx.
type postTurnCtxDoc struct {
	PostTurnCtx postTurnPayload `yaml:"post_turn_ctx"`
}

type runJournalFields struct {
	Scope     string `yaml:"scope"`
	Dir       string `yaml:"dir"`
	Path      string `yaml:"path"`
	SizeBytes int64  `yaml:"size_bytes"`
}

// postTurnPayload is the structured PostTurn context shared with all post-turn sub-agent workflows.
type postTurnPayload struct {
	CorrelationID   string           `yaml:"correlation_id"`
	HostAgentID     string           `yaml:"host_agent_id"`
	SessionSegment  string           `yaml:"session_segment"`
	SessionRoot     string           `yaml:"session_root"`
	InstructionRoot string           `yaml:"instruction_root"`
	UserDataRoot    string           `yaml:"user_data_root"`
	RunJournal      runJournalFields `yaml:"run_journal"`
}

// BuildPostTurnCTXYAML renders PostTurn ctx for agent_task inputs (YAML with top-level post_turn_ctx key).
func BuildPostTurnCTXYAML(rtx *engine.RuntimeContext) (string, error) {
	if rtx == nil {
		return "", fmt.Errorf("wfexec: nil runtime")
	}
	payload, err := buildPostTurnPayload(rtx)
	if err != nil {
		return "", err
	}
	doc := postTurnCtxDoc{PostTurnCtx: payload}
	b, err := yaml.Marshal(&doc)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func buildPostTurnPayload(rtx *engine.RuntimeContext) (postTurnPayload, error) {
	host := runtimeAgentType(rtx)
	if strings.TrimSpace(host) == "" {
		return postTurnPayload{}, fmt.Errorf("wfexec: post_turn ctx: empty host_agent_id")
	}
	sr := strings.TrimSpace(rtx.EffectiveSessionRoot())
	if sr == "" {
		return postTurnPayload{}, fmt.Errorf("wfexec: post_turn ctx: empty session_root")
	}
	corr := strings.TrimSpace(rtx.CorrelationID)
	var p postTurnPayload
	p.CorrelationID = corr
	p.HostAgentID = host
	p.SessionSegment = strings.TrimSpace(rtx.EffectiveSessionSegment())
	p.SessionRoot = sr
	p.InstructionRoot = strings.TrimSpace(rtx.EffectiveInstructionRoot())
	p.UserDataRoot = strings.TrimSpace(rtx.EffectiveUserDataRoot())
	p.RunJournal.Scope = "current_turn"
	p.RunJournal.Dir = filepath.Join(sr, "runs", paths.SanitizeSessionPathSegment(host))
	if corr != "" {
		jp := session.TurnRunJournalPath(sr, host, corr)
		p.RunJournal.Path = jp
		if st, err := os.Stat(jp); err == nil {
			p.RunJournal.SizeBytes = st.Size()
		}
	}
	return p, nil
}
