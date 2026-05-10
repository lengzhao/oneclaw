package memory

import (
	"context"
	"log/slog"
	"strings"

	lzmem "github.com/lengzhao/memory"
	lzservice "github.com/lengzhao/memory/service"
)

const (
	lzRecallTopK          = 15
	lzRecallBodyMaxRunes  = 900
	lzRecallMinConfidence = 0.45
)

// SelectRecall runs github.com/lengzhao/memory FTS recall against agent_memory.sqlite
// (same DB as post-turn / scheduled extract). isolateSessionID must match the workspace
// session id used for extraction ([session.Engine.SessionID] → post-turn isolation).
func SelectRecall(layout Layout, isolateSessionID string, userText string, state *RecallState, budget int) (string, *RecallState) {
	q := strings.TrimSpace(userText)
	if q == "" {
		return "", state.cloneMaps()
	}
	db, err := getAgentMemoryGorm(layout)
	if err != nil {
		slog.Warn("memory.recall.db_open_failed", "err", err)
		return "", state.cloneMaps()
	}
	sid := strings.TrimSpace(isolateSessionID)
	if sid == "" {
		sid = "default"
	}
	ctx := lzservice.WithIsolation(context.Background(), layoutStableTenantID(layout), "default", sid, DefaultRootAgentMemoryAgentID)
	svc := lzmem.NewMemoryService(db)

	excl := surfacedIDsAsSlice(state)
	req := lzservice.RecallRequest{
		Query:          q,
		TopK:           lzRecallTopK,
		MinConfidence:  lzRecallMinConfidence,
		ExcludeItemIDs: excl,
	}
	hits, err := svc.Recall(ctx, req)
	if err != nil {
		slog.Warn("memory.recall.failed", "err", err)
		return "", state.cloneMaps()
	}
	return formatLzMemoryRecallAttachment(hits, state, budget)
}

func surfacedIDsAsSlice(state *RecallState) []string {
	if state == nil || len(state.SurfacedPaths) == 0 {
		return nil
	}
	out := make([]string, 0, len(state.SurfacedPaths))
	for id := range state.SurfacedPaths {
		out = append(out, id)
	}
	return out
}

func truncateRecallDisplay(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if maxRunes <= 0 || s == "" {
		return s
	}
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes]) + "…"
}

func formatLzMemoryRecallAttachment(hits []lzservice.MemoryHit, st *RecallState, budget int) (string, *RecallState) {
	if budget <= 0 {
		budget = MaxSurfacedRecallBytes
	}
	if st == nil {
		st = (&RecallState{}).cloneMaps()
	}
	remaining := budget - st.SurfacedBytes
	if remaining <= 0 {
		return "", st
	}
	header := "Attachment: relevant_memories\n\n"
	if len(header) > remaining {
		return "", st
	}
	var sb strings.Builder
	sb.WriteString(header)
	remaining -= len(header)
	for _, hit := range hits {
		display := truncateRecallDisplay(hit.Summary, lzRecallBodyMaxRunes)
		if display == "" {
			display = truncateRecallDisplay(hit.Content, lzRecallBodyMaxRunes)
		}
		title := strings.TrimSpace(hit.Title)
		line1 := strings.TrimSpace(hit.ID + " [" + string(hit.NamespaceType) + "]")
		if title != "" {
			line1 += " " + title
		}
		block := strings.TrimRight(line1+"\n"+display, "\n")
		if block == "" || display == "" {
			continue
		}
		wrapped := "Memory: " + block + "\n\n"
		if len(wrapped) > remaining {
			break
		}
		sb.WriteString(wrapped)
		remaining -= len(wrapped)
		st.SurfacedPaths[hit.ID] = struct{}{}
		st.SurfacedBytes += len(wrapped)
	}
	out := sb.String()
	if out == header {
		return "", st
	}
	st.SurfacedBytes += len(header)
	return strings.TrimRight(out, "\n"), st
}
