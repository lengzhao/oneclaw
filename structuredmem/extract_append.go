package structuredmem

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	lzmem "github.com/lengzhao/memory"

	"github.com/lengzhao/oneclaw/config"
	mempaths "github.com/lengzhao/oneclaw/memory"
)

// AppendExtractJournal runs lengzhao/memory extraction on dialog text and appends the rendered result
// to memory/<UTC-yyyy-mm>/<UTC-yyyy-mm-dd>.md under instructionRoot.
func AppendExtractJournal(ctx context.Context, instructionRoot string, dialog string, prof config.ModelProfile, sessionSegment, catalogAgentID string, now time.Time) error {
	dialog = strings.TrimSpace(dialog)
	if dialog == "" {
		return nil
	}
	llm := LLMConfigFromProfile(prof)
	if llm == nil {
		slog.InfoContext(ctx, "structuredmem.extract.skip",
			"reason", "no_openai_compatible_llm",
			"profile_id", strings.TrimSpace(prof.ID),
			"provider", strings.TrimSpace(prof.Provider),
		)
		return nil
	}
	db, err := InitDB(instructionRoot)
	if err != nil {
		return err
	}
	defer func() { _ = lzmem.Close(db) }()

	isoCtx := WithIsolationFromTurn(ctx, sessionSegment, catalogAgentID)
	ext := lzmem.NewExtractor(db)
	result, err := ext.Extract(isoCtx, StructuredMemoryExtractRequest(dialog, llm))
	if err != nil {
		return err
	}
	at := now.UTC()
	if err := SyncMemoryMDFromExtract(instructionRoot, result); err != nil {
		slog.WarnContext(ctx, "structuredmem.extract.sync_memory_md_failed", "err", err)
	}

	rel, err := mempaths.NormalizeMemoryMonthRel("")
	if err != nil {
		return err
	}
	abs, err := mempaths.ResolveMemoryMonthMarkdown(instructionRoot, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	md := FormatExtractResultMarkdown(at, result)
	if strings.TrimSpace(md) == "" {
		return nil
	}
	f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, wErr := f.WriteString(md)
	return wErr
}
