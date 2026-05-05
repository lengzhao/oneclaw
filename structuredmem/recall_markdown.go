package structuredmem

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	lzmem "github.com/lengzhao/memory"
)

// RecallHitsMarkdown runs FTS recall and returns numbered hit lines only (no section headings).
func RecallHitsMarkdown(ctx context.Context, instructionRoot, userQuery, sessionSegment, catalogAgentID string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 8000
	}
	q := strings.TrimSpace(userQuery)
	if q == "" {
		return ""
	}
	root := strings.TrimSpace(instructionRoot)
	if root == "" {
		return ""
	}

	db, err := InitDB(root)
	if err != nil {
		slog.WarnContext(ctx, "structuredmem.recall.open_failed", "err", err)
		return ""
	}
	defer func() { _ = lzmem.Close(db) }()

	isoCtx := WithIsolationFromTurn(ctx, sessionSegment, catalogAgentID)
	svc := lzmem.NewMemoryService(db)
	hits, err := svc.Recall(isoCtx, lzmem.RecallRequest{
		Query:         q,
		TopK:          8,
		MinConfidence: 0.45,
	})
	if err != nil {
		slog.WarnContext(ctx, "structuredmem.recall.failed", "err", err)
		return ""
	}
	if len(hits) == 0 {
		return ""
	}

	var b strings.Builder
	for i, h := range hits {
		title := strings.TrimSpace(h.Title)
		if title == "" {
			title = "(untitled)"
		}
		line := fmt.Sprintf("%d. **[%s]** %s — score %.3f\n", i+1, h.NamespaceType, title, h.Score)
		sum := strings.TrimSpace(h.Summary)
		if sum == "" {
			sum = strings.TrimSpace(h.Content)
		}
		if sum != "" {
			if len([]rune(sum)) > 320 {
				sum = string([]rune(sum)[:320]) + "…"
			}
			line += fmt.Sprintf("   - %s\n", sum)
		}
		b.WriteString(line)
	}
	out := strings.TrimRight(b.String(), "\n")
	return truncateRunes(out, maxRunes)
}

// FormatRecallMarkdown wraps RecallHitsMarkdown with a subsection heading for legacy callers / tests.
func FormatRecallMarkdown(ctx context.Context, instructionRoot, userQuery, sessionSegment, catalogAgentID string, maxRunes int) string {
	body := RecallHitsMarkdown(ctx, instructionRoot, userQuery, sessionSegment, catalogAgentID, maxRunes)
	if body == "" {
		return ""
	}
	return "### Structured memory (SQLite / lengzhao/memory)\n\n" + body
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return strings.TrimRight(string(r[:max]), "\n") + "\n\n_(truncated — raise budget.memory_max_runes to show more)_\n"
}
