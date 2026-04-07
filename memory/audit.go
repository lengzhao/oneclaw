package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Append-only audit of writes under memory WriteRoots, project `.oneclaw/rules`,
// user `~/.oneclaw/rules`, and canonical AGENT paths (project `.oneclaw/AGENT.md`, user `~/.oneclaw/AGENT.md`). Log path:
//   <cwd>/.oneclaw/audit/memory-write.jsonl
// Each line is a JSON object: ts, source, path, bytes, sha256.
// Disable with ONCLAW_DISABLE_MEMORY_AUDIT=1/true/yes.
// Teams that version-control .oneclaw can also rely on git history for forensics.

type auditRecord struct {
	TS     string `json:"ts"`
	Source string `json:"source"`
	Path   string `json:"path"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

var auditMu sync.Mutex

func memoryAuditDisabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_MEMORY_AUDIT"))
	return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
}

// AppendMemoryAudit appends one JSON line if absPath is under layout.AuditWriteRoots()
// or matches a canonical AGENT.md path (behavior policy file).
func AppendMemoryAudit(layout Layout, absPath, source string, content []byte) {
	cwd := layout.CWD
	if memoryAuditDisabled() || cwd == "" || absPath == "" {
		return
	}
	absPath = filepath.Clean(absPath)
	if !pathUnderAnyRoot(absPath, layout.AuditWriteRoots()) && !layout.IsBehaviorPolicyFile(absPath) {
		return
	}
	sum := sha256.Sum256(content)
	rec := auditRecord{
		TS:     time.Now().UTC().Format(time.RFC3339Nano),
		Source: source,
		Path:   absPath,
		Bytes:  len(content),
		SHA256: hex.EncodeToString(sum[:]),
	}
	line, err := json.Marshal(rec)
	if err != nil {
		slog.Warn("memory.audit.marshal", "err", err)
		return
	}
	line = append(line, '\n')

	auditPath := filepath.Join(cwd, DotDir, "audit", "memory-write.jsonl")
	auditMu.Lock()
	defer auditMu.Unlock()
	if err := os.MkdirAll(filepath.Dir(auditPath), 0o755); err != nil {
		slog.Warn("memory.audit.mkdir", "path", auditPath, "err", err)
		return
	}
	f, err := os.OpenFile(auditPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		slog.Warn("memory.audit.open", "path", auditPath, "err", err)
		return
	}
	defer f.Close()
	if _, err := f.Write(line); err != nil {
		slog.Warn("memory.audit.write", "path", auditPath, "err", err)
	}
}

func pathUnderAnyRoot(file string, roots []string) bool {
	file = filepath.Clean(file)
	for _, root := range roots {
		if root == "" {
			continue
		}
		if pathUnderRoot(file, filepath.Clean(root)) {
			return true
		}
	}
	return false
}

func pathUnderRoot(file, root string) bool {
	rel, err := filepath.Rel(root, file)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// PathUnderRoot reports whether file lies under root (directory containment, after Clean).
func PathUnderRoot(file, root string) bool {
	return pathUnderRoot(filepath.Clean(file), filepath.Clean(root))
}
