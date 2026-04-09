// Package sessdb provides optional SQLite-backed session metadata and recall state persistence.
package sessdb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/session"
	_ "modernc.org/sqlite" // SQLite driver (pure Go)
)

// Store is a per-project sessions.sqlite database.
type Store struct {
	db *sql.DB
}

// Open creates or opens the SQLite file at path (parent dirs are created).
func Open(path string) (*Store, error) {
	if path == "" {
		return nil, fmt.Errorf("sessdb: empty path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("sessdb: mkdir: %w", err)
	}
	dsn := path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sessdb: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY NOT NULL,
  source TEXT NOT NULL DEFAULT '',
  session_key TEXT NOT NULL DEFAULT '',
  recall_json TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);`)
	if err != nil {
		return fmt.Errorf("sessdb: migrate: %w", err)
	}
	return nil
}

// Close releases the database handle.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

type recallBlob struct {
	Paths         []string `json:"paths"`
	SurfacedBytes int      `json:"surfaced_bytes"`
}

func encodeRecall(st memory.RecallState) ([]byte, error) {
	paths := make([]string, 0, len(st.SurfacedPaths))
	for p := range st.SurfacedPaths {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	return json.Marshal(recallBlob{Paths: paths, SurfacedBytes: st.SurfacedBytes})
}

func decodeRecall(b []byte) (memory.RecallState, error) {
	if len(b) == 0 {
		return memory.RecallState{SurfacedPaths: make(map[string]struct{})}, nil
	}
	var raw recallBlob
	if err := json.Unmarshal(b, &raw); err != nil {
		return memory.RecallState{}, err
	}
	m := make(map[string]struct{}, len(raw.Paths))
	for _, p := range raw.Paths {
		m[p] = struct{}{}
	}
	return memory.RecallState{SurfacedPaths: m, SurfacedBytes: raw.SurfacedBytes}, nil
}

// RecallBridge implements session.RecallPersister for one logical handle.
type RecallBridge struct {
	s   *Store
	src string
	key string
}

var _ session.RecallPersister = (*RecallBridge)(nil)

// NewRecallBridge returns a persister that stores rows under the stable session id.
func NewRecallBridge(st *Store, h session.SessionHandle) session.RecallPersister {
	if st == nil {
		return nil
	}
	return &RecallBridge{s: st, src: h.Source, key: h.SessionKey}
}

// LoadRecall reads recall_json for id.
func (b *RecallBridge) LoadRecall(sessionID string) (memory.RecallState, error) {
	if b == nil || b.s == nil {
		return memory.RecallState{SurfacedPaths: make(map[string]struct{})}, nil
	}
	var raw sql.NullString
	err := b.s.db.QueryRow(`SELECT recall_json FROM sessions WHERE id = ?`, sessionID).Scan(&raw)
	if err == sql.ErrNoRows {
		return memory.RecallState{SurfacedPaths: make(map[string]struct{})}, nil
	}
	if err != nil {
		return memory.RecallState{}, err
	}
	if !raw.Valid || raw.String == "" {
		return memory.RecallState{SurfacedPaths: make(map[string]struct{})}, nil
	}
	return decodeRecall([]byte(raw.String))
}

// SaveRecall upserts the session row and recall_json.
func (b *RecallBridge) SaveRecall(sessionID string, st memory.RecallState) error {
	if b == nil || b.s == nil {
		return nil
	}
	blob, err := encodeRecall(st)
	if err != nil {
		return err
	}
	now := time.Now().Unix()
	_, err = b.s.db.Exec(`
INSERT INTO sessions (id, source, session_key, recall_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
  recall_json = excluded.recall_json,
  updated_at = excluded.updated_at,
  source = excluded.source,
  session_key = excluded.session_key
`, sessionID, b.src, b.key, string(blob), now, now)
	if err != nil {
		return err
	}
	slog.Debug("sessdb.recall_saved", "session_id", sessionID)
	return nil
}
