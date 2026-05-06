package wfexec

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/lengzhao/oneclaw/engine"
	"github.com/lengzhao/oneclaw/paths"
	"github.com/lengzhao/oneclaw/structuredmem"
	"github.com/lengzhao/oneclaw/workflow"
)

type resolvedStructuredExtract struct {
	JournalPath     string
	SessionRoot     string
	HostAgentID     string
	SessionSegment  string
	InstructionRoot string
}

func handleStructuredMemoryExtract(ctx context.Context, in NodeInput, env NodeEnv) (workflow.WorkflowNodeResult, error) {
	rtx := env.Runtime
	if rtx == nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: structured_memory_extract: nil runtime")
	}
	c := ctx
	if c == nil {
		c = context.Background()
	}
	resolved, err := resolveStructuredMemoryExtractInput(strings.TrimSpace(in.Text), rtx)
	if err != nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: structured_memory_extract: %w", err)
	}
	if err := validateRunJournalPath(resolved.JournalPath, resolved.SessionRoot, resolved.HostAgentID); err != nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: structured_memory_extract: %w", err)
	}
	b, err := os.ReadFile(resolved.JournalPath)
	if err != nil {
		return workflow.WorkflowNodeResult{}, fmt.Errorf("wfexec: structured_memory_extract: read journal: %w", err)
	}
	dialog := strings.TrimSpace(string(b))
	if dialog == "" {
		slog.WarnContext(c, "wfexec.structured_memory_extract.empty_journal",
			"path", resolved.JournalPath,
			"host_agent_id", resolved.HostAgentID,
			"correlation_id", strings.TrimSpace(rtx.CorrelationID),
		)
		return workflow.WorkflowNodeResult{Text: "structured_memory_extract: skip empty journal"}, nil
	}
	prof, err := runtimeModelProfile(rtx)
	if err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	if strings.TrimSpace(prof.ID) == "" && strings.TrimSpace(prof.Provider) == "" {
		slog.InfoContext(c, "wfexec.structured_memory_extract.skip_no_llm_profile")
		return workflow.WorkflowNodeResult{Text: "structured_memory_extract: skip no model profile"}, nil
	}
	if err := structuredmem.AppendExtractJournal(
		c,
		resolved.InstructionRoot,
		dialog,
		prof,
		resolved.SessionSegment,
		resolved.HostAgentID,
		time.Now().UTC(),
	); err != nil {
		return workflow.WorkflowNodeResult{}, err
	}
	out := fmt.Sprintf("structured_memory_extract: ok journal=%s", resolved.JournalPath)
	return workflow.WorkflowNodeResult{Text: out}, nil
}

func resolveStructuredMemoryExtractInput(text string, rtx *engine.RuntimeContext) (resolvedStructuredExtract, error) {
	if rtx == nil {
		return resolvedStructuredExtract{}, fmt.Errorf("nil runtime")
	}
	if text != "" {
		if p, ok := parsePostTurnPayloadYAML(text); ok && strings.TrimSpace(p.RunJournal.Path) != "" {
			return mergePayloadWithRuntime(p, rtx), nil
		}
		if r, err := inferStructuredExtractFromJournalPath(text, rtx); err == nil {
			return r, nil
		}
	}
	return resolvedStructuredExtract{}, fmt.Errorf("missing or invalid post_turn_ctx (need YAML with run_journal.path or absolute journal path)")
}

func parsePostTurnPayloadYAML(raw string) (postTurnPayload, bool) {
	var doc postTurnCtxDoc
	if err := yaml.Unmarshal([]byte(raw), &doc); err == nil && payloadLooksPresent(doc.PostTurnCtx) {
		return doc.PostTurnCtx, true
	}
	var flat postTurnPayload
	if err := yaml.Unmarshal([]byte(raw), &flat); err == nil && payloadLooksPresent(flat) {
		return flat, true
	}
	return postTurnPayload{}, false
}

func payloadLooksPresent(p postTurnPayload) bool {
	return strings.TrimSpace(p.HostAgentID) != "" ||
		strings.TrimSpace(p.SessionRoot) != "" ||
		strings.TrimSpace(p.RunJournal.Path) != ""
}

func mergePayloadWithRuntime(p postTurnPayload, rtx *engine.RuntimeContext) resolvedStructuredExtract {
	out := resolvedStructuredExtract{
		JournalPath:     strings.TrimSpace(p.RunJournal.Path),
		SessionRoot:     strings.TrimSpace(p.SessionRoot),
		HostAgentID:     strings.TrimSpace(p.HostAgentID),
		SessionSegment:  strings.TrimSpace(p.SessionSegment),
		InstructionRoot: strings.TrimSpace(p.InstructionRoot),
	}
	if out.SessionSegment == "" {
		out.SessionSegment = strings.TrimSpace(rtx.EffectiveSessionSegment())
	}
	if out.InstructionRoot == "" {
		out.InstructionRoot = strings.TrimSpace(rtx.EffectiveInstructionRoot())
	}
	if out.JournalPath != "" && (out.SessionRoot == "" || out.HostAgentID == "") {
		if inf, err := inferStructuredExtractFromJournalPath(out.JournalPath, rtx); err == nil {
			if out.SessionRoot == "" {
				out.SessionRoot = inf.SessionRoot
			}
			if out.HostAgentID == "" {
				out.HostAgentID = inf.HostAgentID
			}
		}
	}
	return out
}

func inferStructuredExtractFromJournalPath(absPath string, rtx *engine.RuntimeContext) (resolvedStructuredExtract, error) {
	s := strings.TrimSpace(absPath)
	if s == "" || strings.Contains(s, "\n") {
		return resolvedStructuredExtract{}, fmt.Errorf("not a single-line path")
	}
	if !filepath.IsAbs(s) {
		return resolvedStructuredExtract{}, fmt.Errorf("journal path must be absolute")
	}
	s = filepath.Clean(s)
	base := filepath.Base(s)
	if !strings.HasSuffix(strings.ToLower(base), ".jsonl") {
		return resolvedStructuredExtract{}, fmt.Errorf("want .jsonl path")
	}
	agentDir := filepath.Dir(s)
	hostAgent := filepath.Base(agentDir)
	runsDir := filepath.Dir(agentDir)
	if filepath.Base(runsDir) != "runs" {
		return resolvedStructuredExtract{}, fmt.Errorf("path must end with runs/<agent>/<id>.jsonl")
	}
	sessionRoot := filepath.Dir(runsDir)
	hostAgent = paths.SanitizeSessionPathSegment(hostAgent)
	if hostAgent == "" {
		return resolvedStructuredExtract{}, fmt.Errorf("invalid host agent segment")
	}
	return resolvedStructuredExtract{
		JournalPath:     s,
		SessionRoot:     sessionRoot,
		HostAgentID:     hostAgent,
		SessionSegment:  strings.TrimSpace(rtx.EffectiveSessionSegment()),
		InstructionRoot: strings.TrimSpace(rtx.EffectiveInstructionRoot()),
	}, nil
}

func validateRunJournalPath(journalPath, sessionRoot, hostAgentID string) error {
	jp := filepath.Clean(strings.TrimSpace(journalPath))
	sr := filepath.Clean(strings.TrimSpace(sessionRoot))
	host := paths.SanitizeSessionPathSegment(strings.TrimSpace(hostAgentID))
	if jp == "" || sr == "" || host == "" {
		return fmt.Errorf("journal path, session_root, host_agent_id required")
	}
	allowedDir := filepath.Join(sr, "runs", host)
	if rel, err := filepath.Rel(allowedDir, jp); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("journal path outside session runs dir")
	}
	return nil
}
