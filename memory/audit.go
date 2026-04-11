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

	"github.com/lengzhao/oneclaw/rtopts"
)

// Append-only audit of writes under memory WriteRoots, project/user rules dirs,
// and canonical AGENT paths. Log path: <layout.DotOrDataRoot()>/audit/memory-write.jsonl
// Each line is a JSON object: ts, source, path, bytes, sha256.
// Writes under <memory_base>/projects/ (per-repo auto memory and daily logs) are not audited here.
// Disable with features.disable_memory_audit in config.
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
	return rtopts.Current().DisableMemoryAudit
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
	if layout.skipMemoryAuditForGlobalProjectsStore(absPath) {
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

	auditPath := filepath.Join(layout.DotOrDataRoot(), "audit", "memory-write.jsonl")
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

// skipMemoryAuditForGlobalProjectsStore is true when absPath lies under <MemoryBase>/projects/
// (AutoMemoryDir daily logs and topics). Those paths are still normal write roots; audit is omitted by design.
func (l Layout) skipMemoryAuditForGlobalProjectsStore(absPath string) bool {
	mb := filepath.Clean(strings.TrimSpace(l.MemoryBase))
	if mb == "" || mb == "." {
		return false
	}
	projectsRoot := filepath.Join(mb, "projects")
	return pathUnderRoot(absPath, projectsRoot)
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
