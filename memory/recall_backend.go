package memory

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lengzhao/oneclaw/rtopts"
	_ "modernc.org/sqlite"
)

// RecallBackend abstracts recall retrieval while keeping Markdown files as the source of truth.
type RecallBackend interface {
	Name() string
	MarkDirty(ctx context.Context, paths []string) error
	Sync(ctx context.Context, layout Layout, paths []string) error
	Recall(ctx context.Context, req RecallRequest) ([]RecallHit, *RecallState, error)
}

// RecallRequest is the backend input for one recall lookup.
type RecallRequest struct {
	Layout Layout
	Query  string
	Budget int
	State  *RecallState
}

// RecallHit is one structured recall match before attachment formatting.
type RecallHit struct {
	Path        string
	HeadingPath string
	ByteStart   int
	ByteEnd     int
	Text        string
	Score       int
}

const sqliteRecallTopK = 20

type sqliteRecallChunk struct {
	ChunkIndex  int
	HeadingPath string
	ByteStart   int
	ByteEnd     int
	Text        string
	FTSText     string
	ChunkSHA    string
}

type sqliteRecallHitRow struct {
	Path        string
	HeadingPath string
	ByteStart   int
	ByteEnd     int
	Text        string
}

// ScanRecallBackend preserves the legacy on-disk scan behavior behind RecallBackend.
type ScanRecallBackend struct{}

func NewScanRecallBackend() *ScanRecallBackend {
	return &ScanRecallBackend{}
}

// SQLiteRecallBackend is the planned FTS-backed recall backend.
// Until the indexer lands, it conservatively falls back to scan behavior.
type SQLiteRecallBackend struct{}

func NewSQLiteRecallBackend() *SQLiteRecallBackend {
	return &SQLiteRecallBackend{}
}

func selectRecallBackend() RecallBackend {
	switch strings.TrimSpace(rtopts.Current().MemoryRecallBackend) {
	case "", "scan":
		return NewScanRecallBackend()
	case "sqlite":
		return NewSQLiteRecallBackend()
	default:
		return NewScanRecallBackend()
	}
}

func (b *ScanRecallBackend) Name() string { return "scan" }

func (b *ScanRecallBackend) MarkDirty(context.Context, []string) error { return nil }

func (b *ScanRecallBackend) Sync(context.Context, Layout, []string) error { return nil }

func (b *ScanRecallBackend) Recall(_ context.Context, req RecallRequest) ([]RecallHit, *RecallState, error) {
	st := req.State.cloneMaps()
	hits := scanRecallHits(req.Layout, req.Query, st)
	return hits, st, nil
}

func (b *SQLiteRecallBackend) Name() string { return "sqlite" }

func (b *SQLiteRecallBackend) MarkDirty(_ context.Context, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	dbPath, err := runtimeRecallSQLitePath()
	if err != nil {
		return err
	}
	db, err := openRecallSQLiteAtPath(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	now := time.Now().Unix()
	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		if _, err := db.Exec(`INSERT INTO memory_index_meta (
  path, file_sha256, source_mtime_unix, chunk_count, last_indexed_at_unix, dirty, last_error
) VALUES (?, '', 0, 0, ?, 1, '')
ON CONFLICT(path) DO UPDATE SET
  dirty = 1`,
			path, now,
		); err != nil {
			return fmt.Errorf("memory.recall.sqlite: mark dirty %s: %w", path, err)
		}
	}
	return nil
}

func (b *SQLiteRecallBackend) Recall(ctx context.Context, req RecallRequest) ([]RecallHit, *RecallState, error) {
	st := req.State.cloneMaps()
	db, err := openRecallSQLite(req.Layout)
	if err != nil {
		fallback := scanRecallHits(req.Layout, req.Query, st)
		return fallback, st, nil
	}
	defer db.Close()
	if err := pruneSQLiteRecallDeletedPaths(db, req.Layout); err != nil {
		fallback := scanRecallHits(req.Layout, req.Query, st)
		return fallback, st, nil
	}
	syncPaths, err := collectSQLiteRecallSyncPaths(db, req.Layout)
	if err != nil {
		hits := scanRecallHits(req.Layout, req.Query, st)
		return hits, st, nil
	}
	if len(syncPaths) > 0 {
		if err := syncSQLiteRecallPaths(ctx, db, req.Layout, syncPaths); err != nil {
			fallback := scanRecallHits(req.Layout, req.Query, st)
			return fallback, st, nil
		}
	}
	hits, err := querySQLiteRecallHits(db, req.Query, st)
	if err != nil {
		fallback := scanRecallHits(req.Layout, req.Query, st)
		return fallback, st, nil
	}
	if len(hits) == 0 {
		fallback := scanRecallHits(req.Layout, req.Query, st)
		return fallback, st, nil
	}
	return hits, st, nil
}

func openRecallSQLite(layout Layout) (*sql.DB, error) {
	return openRecallSQLiteAtPath(recallSQLitePath(layout))
}

func openRecallSQLiteAtPath(path string) (*sql.DB, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("memory.recall.sqlite: empty path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("memory.recall.sqlite: mkdir: %w", err)
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("memory.recall.sqlite: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := migrateRecallSQLite(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func runtimeRecallSQLitePath() (string, error) {
	p := strings.TrimSpace(rtopts.Current().MemoryRecallSQLitePath)
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	base := strings.TrimSpace(rtopts.Current().MemoryBase)
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("memory.recall.sqlite: resolve user home: %w", err)
		}
		base = filepath.Join(home, DotDir)
	} else {
		home, _ := os.UserHomeDir()
		base = filepath.Clean(expandTilde(home, base))
	}
	if p == "" {
		return filepath.Join(base, "memory", "recall_index.sqlite"), nil
	}
	return filepath.Join(base, p), nil
}

func migrateRecallSQLite(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS memory_chunks (
  id INTEGER PRIMARY KEY,
  path TEXT NOT NULL,
  scope TEXT NOT NULL DEFAULT '',
  memory_kind TEXT NOT NULL DEFAULT '',
  day TEXT NOT NULL DEFAULT '',
  chunk_index INTEGER NOT NULL,
  heading_path TEXT NOT NULL DEFAULT '',
  byte_start INTEGER NOT NULL DEFAULT 0,
  byte_end INTEGER NOT NULL DEFAULT 0,
  text TEXT NOT NULL,
  fts_text TEXT NOT NULL DEFAULT '',
  chunk_sha256 TEXT NOT NULL DEFAULT '',
  file_sha256 TEXT NOT NULL DEFAULT '',
  source_mtime_unix INTEGER NOT NULL DEFAULT 0,
  indexed_at_unix INTEGER NOT NULL DEFAULT 0,
  UNIQUE(path, chunk_index)
);`,
		`CREATE TABLE IF NOT EXISTS memory_index_meta (
  path TEXT PRIMARY KEY NOT NULL,
  file_sha256 TEXT NOT NULL DEFAULT '',
  source_mtime_unix INTEGER NOT NULL DEFAULT 0,
  chunk_count INTEGER NOT NULL DEFAULT 0,
  last_indexed_at_unix INTEGER NOT NULL DEFAULT 0,
  dirty INTEGER NOT NULL DEFAULT 0,
  last_error TEXT NOT NULL DEFAULT ''
);`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS memory_chunks_fts USING fts5(
  fts_text,
  content='memory_chunks',
  content_rowid='id'
);`,
	}
	for _, stmt := range stmts {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("memory.recall.sqlite: migrate: %w", err)
		}
	}
	return nil
}

func (b *SQLiteRecallBackend) Sync(_ context.Context, layout Layout, paths []string) error {
	db, err := openRecallSQLite(layout)
	if err != nil {
		return err
	}
	defer db.Close()
	return syncSQLiteRecallPaths(context.Background(), db, layout, paths)
}

func syncSQLiteRecallPaths(_ context.Context, db *sql.DB, layout Layout, paths []string) error {
	if len(paths) == 0 {
		paths = listMemoryMarkdownFiles(layout)
	}
	for _, path := range paths {
		if err := syncSQLiteRecallPath(db, layout, path); err != nil {
			return err
		}
	}
	return nil
}

func syncSQLiteRecallPath(db *sql.DB, layout Layout, path string) error {
	rawBytes, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("memory.recall.sqlite: read %s: %w", path, err)
	}
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("memory.recall.sqlite: stat %s: %w", path, err)
	}
	raw := string(rawBytes)
	bodyBase := BodyStartByteOffset(raw)
	if bodyBase > len(raw) {
		bodyBase = len(raw)
	}
	body := raw[bodyBase:]
	now := time.Now().Unix()
	chunks := splitSQLiteRecallChunks(raw, bodyBase)
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("memory.recall.sqlite: begin: %w", err)
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id FROM memory_chunks WHERE path = ?`, path)
	if err != nil {
		return fmt.Errorf("memory.recall.sqlite: query old rows: %w", err)
	}
	var oldIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("memory.recall.sqlite: scan old row: %w", err)
		}
		oldIDs = append(oldIDs, id)
	}
	rows.Close()
	for _, id := range oldIDs {
		if _, err := tx.Exec(`DELETE FROM memory_chunks_fts WHERE rowid = ?`, id); err != nil {
			return fmt.Errorf("memory.recall.sqlite: delete old fts row: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM memory_chunks WHERE path = ?`, path); err != nil {
		return fmt.Errorf("memory.recall.sqlite: delete old chunks: %w", err)
	}

	chunkCount := 0
	fileSHA := sha256Hex(body)
	for _, chunk := range chunks {
		res, err := tx.Exec(`INSERT INTO memory_chunks (
  path, scope, memory_kind, day, chunk_index, heading_path, byte_start, byte_end,
  text, fts_text, chunk_sha256, file_sha256, source_mtime_unix, indexed_at_unix
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			path,
			recallScope(layout, path),
			recallMemoryKind(path),
			recallMemoryDay(path),
			chunk.ChunkIndex,
			chunk.HeadingPath,
			chunk.ByteStart,
			chunk.ByteEnd,
			chunk.Text,
			chunk.FTSText,
			chunk.ChunkSHA,
			fileSHA,
			st.ModTime().Unix(),
			now,
		)
		if err != nil {
			return fmt.Errorf("memory.recall.sqlite: insert chunk: %w", err)
		}
		rowID, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("memory.recall.sqlite: last insert id: %w", err)
		}
		if _, err := tx.Exec(`INSERT INTO memory_chunks_fts(rowid, fts_text) VALUES (?, ?)`, rowID, chunk.FTSText); err != nil {
			return fmt.Errorf("memory.recall.sqlite: insert fts row: %w", err)
		}
		chunkCount++
	}
	if _, err := tx.Exec(`INSERT INTO memory_index_meta (
  path, file_sha256, source_mtime_unix, chunk_count, last_indexed_at_unix, dirty, last_error
) VALUES (?, ?, ?, ?, ?, 0, '')
ON CONFLICT(path) DO UPDATE SET
  file_sha256 = excluded.file_sha256,
  source_mtime_unix = excluded.source_mtime_unix,
  chunk_count = excluded.chunk_count,
  last_indexed_at_unix = excluded.last_indexed_at_unix,
  dirty = 0,
  last_error = ''`,
		path,
		sha256Hex(body),
		st.ModTime().Unix(),
		chunkCount,
		now,
	); err != nil {
		return fmt.Errorf("memory.recall.sqlite: upsert meta: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("memory.recall.sqlite: commit: %w", err)
	}
	return nil
}

func splitSQLiteRecallChunks(raw string, bodyBase int) []sqliteRecallChunk {
	if bodyBase > len(raw) {
		bodyBase = len(raw)
	}
	body := raw[bodyBase:]
	if strings.TrimSpace(body) == "" {
		return nil
	}

	type headingState struct {
		path  string
		start int
	}
	var (
		stack        []string
		currentStart = 0
		currentPath  = ""
		chunks       []sqliteRecallChunk
	)

	appendChunk := func(start, end int, headingPath string) {
		if start < 0 {
			start = 0
		}
		if end > len(body) {
			end = len(body)
		}
		if start >= end {
			return
		}
		start = trimSQLiteRecallChunkStart(body, start, end)
		if start >= end {
			return
		}
		text := body[start:end]
		if strings.TrimSpace(text) == "" {
			return
		}
		chunks = append(chunks, sqliteRecallChunk{
			ChunkIndex:  len(chunks),
			HeadingPath: headingPath,
			ByteStart:   bodyBase + start,
			ByteEnd:     bodyBase + end,
			Text:        text,
			FTSText:     recallFTSText(text),
			ChunkSHA:    sha256Hex(text),
		})
	}

	for lineStart := 0; lineStart < len(body); {
		lineEnd := strings.IndexByte(body[lineStart:], '\n')
		nextStart := len(body)
		line := body[lineStart:]
		if lineEnd >= 0 {
			line = body[lineStart : lineStart+lineEnd]
			nextStart = lineStart + lineEnd + 1
		}
		if level, title, ok := parseMarkdownHeadingLine(line); ok {
			if lineStart > currentStart {
				appendChunk(currentStart, lineStart, currentPath)
			}
			if level < 1 {
				level = 1
			}
			if level > len(stack)+1 {
				level = len(stack) + 1
			}
			if level-1 < len(stack) {
				stack = stack[:level-1]
			}
			stack = append(stack, title)
			currentPath = strings.Join(stack, " / ")
			currentStart = lineStart
		}
		lineStart = nextStart
	}
	appendChunk(currentStart, len(body), currentPath)
	return chunks
}

func parseMarkdownHeadingLine(line string) (level int, title string, ok bool) {
	line = strings.TrimRight(line, "\r")
	if line == "" || line[0] != '#' {
		return 0, "", false
	}
	for level < len(line) && line[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	if level >= len(line) || line[level] != ' ' {
		return 0, "", false
	}
	title = strings.TrimSpace(line[level+1:])
	if title == "" {
		return 0, "", false
	}
	return level, title, true
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func recallScope(layout Layout, path string) string {
	switch {
	case PathUnderRoot(path, layout.User):
		return "user"
	case PathUnderRoot(path, layout.Project):
		return "project"
	case PathUnderRoot(path, layout.TeamUser):
		return "team_user"
	case PathUnderRoot(path, layout.TeamProject):
		return "team_project"
	case PathUnderRoot(path, layout.Auto):
		return "auto"
	default:
		return ""
	}
}

func recallMemoryKind(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if _, err := time.Parse("2006-01-02", base); err == nil {
		return "daily"
	}
	return "note"
}

func recallMemoryDay(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	if _, err := time.Parse("2006-01-02", base); err == nil {
		return base
	}
	return ""
}

func trimSQLiteRecallChunkStart(body string, start, end int) int {
	for start < end {
		switch body[start] {
		case '\n', '\r':
			start++
		default:
			return start
		}
	}
	return start
}

func pruneSQLiteRecallDeletedPaths(db *sql.DB, layout Layout) error {
	live := make(map[string]struct{})
	for _, path := range listMemoryMarkdownFiles(layout) {
		live[path] = struct{}{}
	}
	rows, err := db.Query(`SELECT path FROM memory_index_meta`)
	if err != nil {
		return fmt.Errorf("memory.recall.sqlite: query indexed paths: %w", err)
	}
	defer rows.Close()

	var deleted []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return fmt.Errorf("memory.recall.sqlite: scan indexed path: %w", err)
		}
		if _, ok := live[path]; !ok {
			deleted = append(deleted, path)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("memory.recall.sqlite: iterate indexed paths: %w", err)
	}
	for _, path := range deleted {
		if err := deleteSQLiteRecallPath(db, path); err != nil {
			return err
		}
	}
	return nil
}

func collectSQLiteRecallSyncPaths(db *sql.DB, layout Layout) ([]string, error) {
	candidates := listMemoryMarkdownFiles(layout)
	if len(candidates) == 0 {
		return nil, nil
	}
	needsSync := make([]string, 0, len(candidates))
	for _, path := range candidates {
		need, err := sqliteRecallPathNeedsSync(db, path)
		if err != nil {
			return nil, err
		}
		if need {
			needsSync = append(needsSync, path)
		}
	}
	return needsSync, nil
}

func sqliteRecallPathNeedsSync(db *sql.DB, path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("memory.recall.sqlite: stat sync candidate %s: %w", path, err)
	}
	var (
		sourceMTime int64
		dirty       int
	)
	err = db.QueryRow(`SELECT source_mtime_unix, dirty FROM memory_index_meta WHERE path = ?`, path).Scan(&sourceMTime, &dirty)
	if err == sql.ErrNoRows {
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("memory.recall.sqlite: query meta %s: %w", path, err)
	}
	if dirty != 0 {
		return true, nil
	}
	if sourceMTime != st.ModTime().Unix() {
		return true, nil
	}
	return false, nil
}

func deleteSQLiteRecallPath(db *sql.DB, path string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("memory.recall.sqlite: begin delete %s: %w", path, err)
	}
	defer tx.Rollback()
	rows, err := tx.Query(`SELECT id FROM memory_chunks WHERE path = ?`, path)
	if err != nil {
		return fmt.Errorf("memory.recall.sqlite: query old rows: %w", err)
	}
	var oldIDs []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return fmt.Errorf("memory.recall.sqlite: scan old row: %w", err)
		}
		oldIDs = append(oldIDs, id)
	}
	rows.Close()
	for _, id := range oldIDs {
		if _, err := tx.Exec(`DELETE FROM memory_chunks_fts WHERE rowid = ?`, id); err != nil {
			return fmt.Errorf("memory.recall.sqlite: delete old fts row: %w", err)
		}
	}
	if _, err := tx.Exec(`DELETE FROM memory_chunks WHERE path = ?`, path); err != nil {
		return fmt.Errorf("memory.recall.sqlite: delete old chunks: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM memory_index_meta WHERE path = ?`, path); err != nil {
		return fmt.Errorf("memory.recall.sqlite: delete meta: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("memory.recall.sqlite: commit delete %s: %w", path, err)
	}
	return nil
}

func querySQLiteRecallHits(db *sql.DB, userText string, st *RecallState) ([]RecallHit, error) {
	match := compileSQLiteRecallMatch(userText)
	if match == "" {
		return nil, nil
	}
	rows, err := db.Query(`
SELECT c.path, c.heading_path, c.byte_start, c.byte_end, c.text
FROM memory_chunks_fts f
JOIN memory_chunks c ON c.id = f.rowid
WHERE memory_chunks_fts MATCH ?
ORDER BY bm25(memory_chunks_fts), c.id
LIMIT ?`, match, sqliteRecallTopK)
	if err != nil {
		return nil, fmt.Errorf("memory.recall.sqlite: query hits: %w", err)
	}
	defer rows.Close()
	var rawHits []sqliteRecallHitRow
	for rows.Next() {
		var hit sqliteRecallHitRow
		if err := rows.Scan(&hit.Path, &hit.HeadingPath, &hit.ByteStart, &hit.ByteEnd, &hit.Text); err != nil {
			return nil, fmt.Errorf("memory.recall.sqlite: scan hit: %w", err)
		}
		rawHits = append(rawHits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("memory.recall.sqlite: iterate hits: %w", err)
	}
	terms := tokenizeRecall(userText)
	hits := rerankSQLiteRecallHits(rawHits, terms)
	var out []RecallHit
	for _, hit := range hits {
		if _, dup := st.SurfacedPaths[hit.Path]; dup {
			continue
		}
		out = append(out, hit)
	}
	return out, nil
}

func recallFTSText(body string) string {
	terms := tokenizeRecall(body)
	if len(terms) == 0 {
		return strings.TrimSpace(strings.ToLower(body))
	}
	return strings.Join(terms, " ")
}

func compileSQLiteRecallMatch(userText string) string {
	terms := tokenizeRecall(userText)
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " AND ")
}

func formatSQLiteRecallHitText(headingPath string, byteStart int, text string) string {
	var sb strings.Builder
	if headingPath != "" {
		sb.WriteString("- heading: ")
		sb.WriteString(headingPath)
		sb.WriteByte('\n')
	}
	sb.WriteString("- offset ")
	sb.WriteString(strconv.Itoa(byteStart))
	sb.WriteString(" (file bytes): ")
	sb.WriteString(truncateOneLineSnippet(text, maxSnippetLineRunes))
	sb.WriteByte('\n')
	return sb.String()
}

func sqliteRecallHitBoost(hit sqliteRecallHitRow, terms []string) int {
	score := 0
	pathLower := strings.ToLower(filepath.Base(hit.Path))
	headingLower := strings.ToLower(hit.HeadingPath)
	textLower := strings.ToLower(hit.Text)
	for _, term := range terms {
		if term == "" {
			continue
		}
		tl := strings.ToLower(term)
		if strings.Contains(pathLower, tl) {
			score += 4
		}
		if strings.Contains(headingLower, tl) {
			score += 3
		}
		if strings.Contains(textLower, tl) {
			score += 1
		}
	}
	return score
}

func rerankSQLiteRecallHits(rawHits []sqliteRecallHitRow, terms []string) []RecallHit {
	type scored struct {
		raw   sqliteRecallHitRow
		score int
	}
	scoredHits := make([]scored, 0, len(rawHits))
	for _, hit := range rawHits {
		scoredHits = append(scoredHits, scored{
			raw:   hit,
			score: sqliteRecallHitBoost(hit, terms),
		})
	}
	sort.SliceStable(scoredHits, func(i, j int) bool {
		if scoredHits[i].score != scoredHits[j].score {
			return scoredHits[i].score > scoredHits[j].score
		}
		if scoredHits[i].raw.Path != scoredHits[j].raw.Path {
			return scoredHits[i].raw.Path < scoredHits[j].raw.Path
		}
		return scoredHits[i].raw.ByteStart < scoredHits[j].raw.ByteStart
	})
	seenPaths := make(map[string]struct{}, len(scoredHits))
	out := make([]RecallHit, 0, len(scoredHits))
	for _, hit := range scoredHits {
		if _, ok := seenPaths[hit.raw.Path]; ok {
			continue
		}
		seenPaths[hit.raw.Path] = struct{}{}
		out = append(out, RecallHit{
			Path:        hit.raw.Path,
			HeadingPath: hit.raw.HeadingPath,
			ByteStart:   hit.raw.ByteStart,
			ByteEnd:     hit.raw.ByteEnd,
			Text:        formatSQLiteRecallHitText(hit.raw.HeadingPath, hit.raw.ByteStart, hit.raw.Text),
			Score:       hit.score,
		})
	}
	return out
}

func formatRecallAttachment(hits []RecallHit, st *RecallState, budget int) (string, *RecallState) {
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
		block := strings.TrimRight(hit.Text, "\n")
		if block == "" {
			continue
		}
		wrapped := "Memory: " + hit.Path + "\n" + block + "\n\n"
		if len(wrapped) > remaining {
			break
		}
		sb.WriteString(wrapped)
		remaining -= len(wrapped)
		st.SurfacedPaths[hit.Path] = struct{}{}
		st.SurfacedBytes += len(wrapped)
	}
	out := sb.String()
	if out == header {
		return "", st
	}
	st.SurfacedBytes += len(header)
	return strings.TrimRight(out, "\n"), st
}
