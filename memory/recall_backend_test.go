package memory

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lengzhao/oneclaw/rtopts"
)

func setSQLiteRuntimeForTest(t *testing.T, lay Layout) {
	t.Helper()
	s := rtopts.DefaultSnapshot()
	s.MemoryBase = lay.MemoryBase
	s.MemoryRecallBackend = "sqlite"
	s.MemoryRecallSQLitePath = "memory/test-recall.sqlite"
	rtopts.Set(&s)
}

func TestScanRecallBackend_RecallReturnsStructuredHits(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	proj := lay.Project
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(proj, "episodic.md")
	if err := os.WriteFile(path, []byte("recall_backend_unique_token here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := NewScanRecallBackend()
	hits, st, err := backend.Recall(t.Context(), RecallRequest{
		Layout: lay,
		Query:  "recall_backend_unique_token",
		Budget: 12_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		t.Fatal("want updated recall state")
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if hits[0].Path != path {
		t.Fatalf("hit path = %q, want %q", hits[0].Path, path)
	}
	if strings.Contains(hits[0].Text, "Attachment: relevant_memories") {
		t.Fatalf("backend should return structured hit text, got attachment block:\n%s", hits[0].Text)
	}
	if !strings.Contains(hits[0].Text, "recall_backend_unique_token") {
		t.Fatalf("hit text missing token:\n%s", hits[0].Text)
	}
}

func TestSelectRecallBackend_DefaultSqliteAndUnknownSelection(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	if got := selectRecallBackend().Name(); got != "scan" {
		t.Fatalf("default backend = %q, want scan", got)
	}

	s := rtopts.DefaultSnapshot()
	s.MemoryRecallBackend = "sqlite"
	rtopts.Set(&s)
	if got := selectRecallBackend().Name(); got != "sqlite" {
		t.Fatalf("sqlite backend should be selectable, got %q", got)
	}

	s.MemoryRecallBackend = "unknown"
	rtopts.Set(&s)
	if got := selectRecallBackend().Name(); got != "scan" {
		t.Fatalf("unknown backend should fall back to scan, got %q", got)
	}
}

func TestSQLiteRecallBackend_RecallFallsBackToScan(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	proj := lay.Project
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(proj, "sqlite-fallback.md")
	if err := os.WriteFile(path, []byte("sqlite_backend_unique_token here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	backend := NewSQLiteRecallBackend()
	hits, st, err := backend.Recall(t.Context(), RecallRequest{
		Layout: lay,
		Query:  "sqlite_backend_unique_token",
		Budget: 12_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		t.Fatal("want updated recall state")
	}
	if backend.Name() != "sqlite" {
		t.Fatalf("backend name = %q, want sqlite", backend.Name())
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if hits[0].Path != path {
		t.Fatalf("hit path = %q, want %q", hits[0].Path, path)
	}
	if !strings.Contains(hits[0].Text, "sqlite_backend_unique_token") {
		t.Fatalf("hit text missing token:\n%s", hits[0].Text)
	}
}

func TestSQLiteRecallBackend_SyncCreatesDatabaseAndTables(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)

	setSQLiteRuntimeForTest(t, lay)

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, nil); err != nil {
		t.Fatal(err)
	}

	dbPath := recallSQLitePath(lay)
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("sqlite db not created: %v", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	for _, table := range []string{"memory_chunks", "memory_index_meta", "memory_chunks_fts"} {
		var name string
		if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type IN ('table', 'virtual table') AND name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}

func TestSQLiteRecallBackend_SyncIndexesProvidedMarkdownFile(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(lay.Project, "indexed.md")
	raw := "---\ntitle: example\n---\n\nhello sqlite sync\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	setSQLiteRuntimeForTest(t, lay)

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, []string{path}); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", recallSQLitePath(lay))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var gotPath string
	var chunkIndex int
	var text string
	if err := db.QueryRow(`SELECT path, chunk_index, text FROM memory_chunks WHERE path = ?`, path).Scan(&gotPath, &chunkIndex, &text); err != nil {
		t.Fatalf("query chunk: %v", err)
	}
	if gotPath != path {
		t.Fatalf("chunk path = %q, want %q", gotPath, path)
	}
	if chunkIndex != 0 {
		t.Fatalf("chunk_index = %d, want 0", chunkIndex)
	}
	if text != "hello sqlite sync\n" {
		t.Fatalf("chunk text = %q", text)
	}

	var chunkCount int
	var dirty int
	if err := db.QueryRow(`SELECT chunk_count, dirty FROM memory_index_meta WHERE path = ?`, path).Scan(&chunkCount, &dirty); err != nil {
		t.Fatalf("query meta: %v", err)
	}
	if chunkCount != 1 {
		t.Fatalf("chunk_count = %d, want 1", chunkCount)
	}
	if dirty != 0 {
		t.Fatalf("dirty = %d, want 0", dirty)
	}

	var ftsCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_chunks_fts WHERE rowid IN (SELECT id FROM memory_chunks WHERE path = ?)`, path).Scan(&ftsCount); err != nil {
		t.Fatalf("query fts: %v", err)
	}
	if ftsCount != 1 {
		t.Fatalf("fts row count = %d, want 1", ftsCount)
	}
}

func TestSQLiteRecallBackend_RecallRemovesDeletedIndexedContent(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(lay.Project, "indexed-recall.md")
	if err := os.WriteFile(path, []byte("sqlite indexed recall token\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	setSQLiteRuntimeForTest(t, lay)

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, []string{path}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	hits, st, err := backend.Recall(t.Context(), RecallRequest{
		Layout: lay,
		Query:  "indexed recall",
		Budget: 12_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if st == nil {
		t.Fatal("want updated recall state")
	}
	if len(hits) != 0 {
		t.Fatalf("want deleted file to disappear from recall, got %#v", hits)
	}

	db, err := sql.Open("sqlite", recallSQLitePath(lay))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var metaCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_index_meta WHERE path = ?`, path).Scan(&metaCount); err != nil {
		t.Fatal(err)
	}
	if metaCount != 0 {
		t.Fatalf("metaCount = %d, want 0", metaCount)
	}

	var chunkCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM memory_chunks WHERE path = ?`, path).Scan(&chunkCount); err != nil {
		t.Fatal(err)
	}
	if chunkCount != 0 {
		t.Fatalf("chunkCount = %d, want 0", chunkCount)
	}
}

func TestSQLiteRecallBackend_SyncSplitsMarkdownByHeading(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(lay.Project, "heading-chunks.md")
	raw := "# Alpha\n\nalpha body\n\n## Beta\n\nbeta body\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	setSQLiteRuntimeForTest(t, lay)

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, []string{path}); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", recallSQLitePath(lay))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	rows, err := db.Query(`SELECT chunk_index, heading_path, text FROM memory_chunks WHERE path = ? ORDER BY chunk_index`, path)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()

	type row struct {
		ChunkIndex  int
		HeadingPath string
		Text        string
	}
	var got []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.ChunkIndex, &r.HeadingPath, &r.Text); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 chunks, got %d: %#v", len(got), got)
	}
	if got[0].ChunkIndex != 0 || got[0].HeadingPath != "Alpha" || !strings.Contains(got[0].Text, "alpha body") {
		t.Fatalf("chunk 0 = %#v", got[0])
	}
	if got[1].ChunkIndex != 1 || got[1].HeadingPath != "Alpha / Beta" || !strings.Contains(got[1].Text, "beta body") {
		t.Fatalf("chunk 1 = %#v", got[1])
	}
}

func TestSQLiteRecallBackend_RecallReturnsMatchingHeadingChunk(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(lay.Project, "heading-recall.md")
	raw := "# Build\n\nbuild notes only\n\n## Deploy\n\ndeploy marker unique token\n"
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}

	setSQLiteRuntimeForTest(t, lay)

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, []string{path}); err != nil {
		t.Fatal(err)
	}

	hits, _, err := backend.Recall(t.Context(), RecallRequest{
		Layout: lay,
		Query:  "deploy marker",
		Budget: 12_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if hits[0].HeadingPath != "Build / Deploy" {
		t.Fatalf("heading path = %q", hits[0].HeadingPath)
	}
	if hits[0].ByteStart <= 0 {
		t.Fatalf("byte start = %d", hits[0].ByteStart)
	}
	if !strings.Contains(hits[0].Text, "deploy marker unique token") {
		t.Fatalf("hit text = %q", hits[0].Text)
	}
	if !strings.Contains(hits[0].Text, "heading: Build / Deploy") {
		t.Fatalf("hit should include heading path, got %q", hits[0].Text)
	}
	if !strings.Contains(hits[0].Text, "offset ") {
		t.Fatalf("hit should include byte offset, got %q", hits[0].Text)
	}
	if strings.Contains(hits[0].Text, "build notes only") {
		t.Fatalf("hit should be matching chunk only, got %q", hits[0].Text)
	}
}

func TestSQLiteRecallBackend_RecallRefreshesStaleIndexedFile(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(lay.Project, "stale-refresh.md")
	if err := os.WriteFile(path, []byte("old token only\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	setSQLiteRuntimeForTest(t, lay)

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, []string{path}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("new token only\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hits, _, err := backend.Recall(t.Context(), RecallRequest{
		Layout: lay,
		Query:  "new token",
		Budget: 12_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 {
		t.Fatalf("want 1 hit, got %d", len(hits))
	}
	if !strings.Contains(hits[0].Text, "new token only") {
		t.Fatalf("hit text = %q", hits[0].Text)
	}
	if strings.Contains(hits[0].Text, "old token only") {
		t.Fatalf("stale indexed content still returned: %q", hits[0].Text)
	}
}

func TestSQLiteRecallHitBoostPrefersHeadingAndFilenameMatches(t *testing.T) {
	terms := []string{"deploy", "marker"}
	hit := sqliteRecallHitRow{
		Path:        "/tmp/proj/deploy-notes.md",
		HeadingPath: "Build / Deploy",
		Text:        "deploy marker unique token",
	}
	got := sqliteRecallHitBoost(hit, terms)
	if got <= 0 {
		t.Fatalf("sqliteRecallHitBoost = %d, want positive", got)
	}

	plain := sqliteRecallHitRow{
		Path:        "/tmp/proj/notes.md",
		HeadingPath: "General",
		Text:        "deploy marker unique token",
	}
	if got <= sqliteRecallHitBoost(plain, terms) {
		t.Fatalf("expected heading+filename boosted hit to outrank plain hit")
	}
}

func TestRerankSQLiteRecallHits_LimitsDuplicatePaths(t *testing.T) {
	terms := []string{"deploy"}
	in := []sqliteRecallHitRow{
		{Path: "/tmp/a.md", HeadingPath: "Deploy", ByteStart: 10, Text: "deploy first"},
		{Path: "/tmp/a.md", HeadingPath: "Deploy / More", ByteStart: 20, Text: "deploy second"},
		{Path: "/tmp/b.md", HeadingPath: "Deploy", ByteStart: 30, Text: "deploy third"},
	}
	got := rerankSQLiteRecallHits(in, terms)
	if len(got) != 2 {
		t.Fatalf("want 2 hits after per-path cap, got %d", len(got))
	}
	paths := []string{got[0].Path, got[1].Path}
	slices.Sort(paths)
	if !slices.Equal(paths, []string{"/tmp/a.md", "/tmp/b.md"}) {
		t.Fatalf("paths = %#v", paths)
	}
}

func TestSQLiteRecallBackend_MarkDirtySetsMetaDirty(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(lay.Project, "dirty.md")
	if err := os.WriteFile(path, []byte("dirty me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	setSQLiteRuntimeForTest(t, lay)

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, []string{path}); err != nil {
		t.Fatal(err)
	}
	if err := backend.MarkDirty(t.Context(), []string{path}); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", recallSQLitePath(lay))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var dirty int
	if err := db.QueryRow(`SELECT dirty FROM memory_index_meta WHERE path = ?`, path).Scan(&dirty); err != nil {
		t.Fatal(err)
	}
	if dirty != 1 {
		t.Fatalf("dirty = %d, want 1", dirty)
	}
}

func TestSQLiteRecallBackend_RecallSyncsOnlyDirtyOrStalePaths(t *testing.T) {
	t.Cleanup(func() { rtopts.Set(nil) })

	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	if err := os.MkdirAll(lay.Project, 0o755); err != nil {
		t.Fatal(err)
	}
	pathA := filepath.Join(lay.Project, "a.md")
	pathB := filepath.Join(lay.Project, "b.md")
	if err := os.WriteFile(pathA, []byte("alpha keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte("beta old\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	setSQLiteRuntimeForTest(t, lay)

	backend := NewSQLiteRecallBackend()
	if err := backend.Sync(t.Context(), lay, []string{pathA, pathB}); err != nil {
		t.Fatal(err)
	}

	db, err := sql.Open("sqlite", recallSQLitePath(lay))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var beforeA, beforeB int64
	if err := db.QueryRow(`SELECT last_indexed_at_unix FROM memory_index_meta WHERE path = ?`, pathA).Scan(&beforeA); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT last_indexed_at_unix FROM memory_index_meta WHERE path = ?`, pathB).Scan(&beforeB); err != nil {
		t.Fatal(err)
	}

	time.Sleep(1100 * time.Millisecond)
	if err := os.WriteFile(pathB, []byte("beta new token\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	hits, _, err := backend.Recall(t.Context(), RecallRequest{
		Layout: lay,
		Query:  "new token",
		Budget: 12_000,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || !strings.Contains(hits[0].Text, "beta new token") {
		t.Fatalf("hits = %#v", hits)
	}

	var afterA, afterB int64
	var dirtyB int
	if err := db.QueryRow(`SELECT last_indexed_at_unix FROM memory_index_meta WHERE path = ?`, pathA).Scan(&afterA); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT last_indexed_at_unix, dirty FROM memory_index_meta WHERE path = ?`, pathB).Scan(&afterB, &dirtyB); err != nil {
		t.Fatal(err)
	}
	if afterA != beforeA {
		t.Fatalf("unchanged path should not resync: before=%d after=%d", beforeA, afterA)
	}
	if afterB <= beforeB {
		t.Fatalf("stale path should resync: before=%d after=%d", beforeB, afterB)
	}
	if dirtyB != 0 {
		t.Fatalf("dirtyB = %d, want 0", dirtyB)
	}
}
