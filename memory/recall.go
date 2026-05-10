package memory

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	lzmem "github.com/lengzhao/memory"
	lzservice "github.com/lengzhao/memory/service"
)

const (
	lzRecallTopK          = 15
	lzRecallBodyMaxRunes  = 900
	lzRecallMinConfidence = 0.45
)

func mergeRecallHits(a, b []lzservice.MemoryHit, topK int) []lzservice.MemoryHit {
	if topK <= 0 {
		topK = lzRecallTopK
	}
	byID := make(map[string]lzservice.MemoryHit)
	for _, h := range a {
		byID[h.ID] = h
	}
	for _, h := range b {
		old, ok := byID[h.ID]
		if !ok || h.Score > old.Score {
			byID[h.ID] = h
		}
	}
	out := make([]lzservice.MemoryHit, 0, len(byID))
	for _, h := range byID {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	if len(out) > topK {
		out = out[:topK]
	}
	return out
}

// SelectRecall runs github.com/lengzhao/memory FTS recall against agent_memory.sqlite
// (same DB as post-turn / scheduled extract). isolateSessionID must match the workspace
// session id used for extraction ([session.Engine.SessionID] → post-turn isolation).
// Hits from [ScheduledMaintainIsolationSessionID] are merged in so scheduled consolidation
// remains visible alongside per-session memories (namespaces differ by session in transient storage).
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
	tenant := layoutStableTenantID(layout)
	ctxSession := lzservice.WithIsolation(context.Background(), tenant, "default", sid, DefaultRootAgentMemoryAgentID)
	svc := lzmem.NewMemoryService(db)

	excl := surfacedIDsAsSlice(state)
	req := lzservice.RecallRequest{
		Query:          q,
		TopK:           lzRecallTopK,
		MinConfidence:  lzRecallMinConfidence,
		ExcludeItemIDs: excl,
	}

	hitsSession, errSession := svc.Recall(ctxSession, req)

	var hitsScheduled []lzservice.MemoryHit
	var errScheduled error
	if sid != ScheduledMaintainIsolationSessionID {
		ctxSched := lzservice.WithIsolation(context.Background(), tenant, "default", ScheduledMaintainIsolationSessionID, DefaultRootAgentMemoryAgentID)
		hitsScheduled, errScheduled = svc.Recall(ctxSched, req)
		if errScheduled != nil {
			slog.Warn("memory.recall.scheduled_scope_failed", "err", errScheduled)
		}
	}

	var hits []lzservice.MemoryHit
	switch {
	case errSession != nil && errScheduled != nil:
		slog.Warn("memory.recall.failed", "err_session", errSession, "err_scheduled", errScheduled)
		return "", state.cloneMaps()
	case errSession != nil:
		hits = hitsScheduled
	case errScheduled != nil || sid == ScheduledMaintainIsolationSessionID:
		hits = hitsSession
	default:
		hits = mergeRecallHits(hitsSession, hitsScheduled, lzRecallTopK)
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

// formatLzMemoryRecallAttachment builds recall text; budget is UTF-8 bytes (see [ApplyTurnBudget] if recall is truncated afterward).
func formatLzMemoryRecallAttachment(hits []lzservice.MemoryHit, st *RecallState, budget int) (string, *RecallState) {
	if budget <= 0 {
		budget = MaxSurfacedRecallBytes
	}
	if st == nil {
		st = (&RecallState{}).cloneMaps()
	} else if st.SurfacedPaths == nil {
		st.SurfacedPaths = make(map[string]struct{})
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
