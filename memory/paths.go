// Package memory implements file-based memory discovery, injection, and helpers for phase B.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/rtopts"
)

// DotDir is the per-project configuration directory for oneclaw.
const DotDir = ".oneclaw"

// AgentInstructionsFile is the repo/user instructions entry filename.
const AgentInstructionsFile = "AGENT.md"

const entrypointName = "MEMORY.md"

// expandTilde replaces a leading "~/" or "~\" (and bare "~") with home. Other paths are unchanged.
// Shells expand ~ in .env files, but many loaders pass the string through literally; Go treats "~" as a normal path segment.
func expandTilde(home, p string) string {
	if home == "" || p == "" {
		return p
	}
	if p == "~" {
		return home
	}
	if len(p) >= 2 && p[0] == '~' {
		sep := p[1]
		if sep == filepath.Separator || sep == '/' || sep == '\\' {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// MemoryBaseDir resolves the base config/memory directory (~/.oneclaw or paths.memory_base from config).
func MemoryBaseDir(home string) string {
	if v := strings.TrimSpace(rtopts.Current().MemoryBase); v != "" {
		return filepath.Clean(expandTilde(home, v))
	}
	return filepath.Join(home, DotDir)
}

// AutoMemoryDir is the per-project auto memory directory (<base>/projects/<slug>/memory).
func AutoMemoryDir(cwd, memoryBase string) string {
	slug := projectSlug(cwd)
	return filepath.Join(memoryBase, "projects", slug, "memory")
}

func projectSlug(cwd string) string {
	clean := filepath.Clean(cwd)
	sum := sha256.Sum256([]byte(clean))
	short := hex.EncodeToString(sum[:6])
	base := filepath.Base(clean)
	if base == "." || base == "/" {
		base = "root"
	}
	return sanitizeDirName(base) + "_" + short
}

func sanitizeDirName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "project"
	}
	return out
}

// UserMemoryDir returns ~/.oneclaw/memory (or under memory base).
func UserMemoryDir(memoryBase string) string {
	return filepath.Join(memoryBase, "memory")
}

// ProjectMemoryDir returns <cwd>/.oneclaw/memory.
func ProjectMemoryDir(cwd string) string {
	return filepath.Join(cwd, DotDir, "memory")
}

// ProjectMemoryMdPath returns <cwd>/.oneclaw/memory/MEMORY.md (project entrypoint).
func ProjectMemoryMdPath(cwd string) string {
	return filepath.Join(ProjectMemoryDir(cwd), entrypointName)
}

// ProjectEpisodeDailyPath returns <cwd>/.oneclaw/memory/YYYY-MM-DD.md (dateStr at least 10 chars).
// Auto-maintained episodic digests live next to MEMORY.md; recall includes these files and skips MEMORY.md at the root.
func ProjectEpisodeDailyPath(cwd, dateYYYYMMDD string) string {
	dateYYYYMMDD = strings.TrimSpace(dateYYYYMMDD)
	if len(dateYYYYMMDD) >= 10 {
		dateYYYYMMDD = dateYYYYMMDD[:10]
	}
	return filepath.Join(ProjectMemoryDir(cwd), dateYYYYMMDD+".md")
}

// AgentMemoryDir returns the on-disk directory for an agent type and scope.
func AgentMemoryDir(cwd, memoryBase, agentType string, scope AgentScope) string {
	dir := sanitizeDirName(agentType)
	switch scope {
	case AgentScopeUser:
		return filepath.Join(memoryBase, "agent-memory", dir)
	case AgentScopeProject:
		return filepath.Join(cwd, DotDir, "agent-memory", dir)
	default:
		return filepath.Join(cwd, DotDir, "agent-memory", dir)
	}
}

// TeamMemoryDirUser is team-shared memory under the global config base.
func TeamMemoryDirUser(memoryBase string) string {
	return filepath.Join(memoryBase, "team-memory")
}

// TeamMemoryDirProject is team memory checked in under the repo.
func TeamMemoryDirProject(cwd string) string {
	return filepath.Join(cwd, DotDir, "team-memory")
}

// DailyLogPath returns <autoMemoryDir>/logs/YYYY/MM/YYYY-MM-DD.md for the given date (date "2006-01-02").
func DailyLogPath(autoMemoryDir, date string) string {
	if len(date) < 10 {
		date = date[:0]
	}
	y, m, d := "0000", "00", "00"
	if len(date) >= 10 {
		y, m, d = date[0:4], date[5:7], date[8:10]
	}
	name := fmt.Sprintf("%s-%s-%s.md", y, m, d)
	return filepath.Join(autoMemoryDir, "logs", y, m, name)
}

// DialogHistoryPath returns <cwd>/.oneclaw/memory/YYYY-MM-DD/dialog_history.json (calendar date, local).
// Prefer DialogHistoryPathForSession when a stable session id is available so concurrent chats do not share one file.
func (l Layout) DialogHistoryPath(date string) string {
	date = strings.TrimSpace(date)
	if len(date) >= 10 {
		date = date[:10]
	}
	return filepath.Join(l.Project, date, "dialog_history.json")
}

// DialogHistoryPathForSession returns per-session dialog history under the day's directory.
// sessionID should be a stable filesystem-safe segment (e.g. StableSessionID from session package).
func (l Layout) DialogHistoryPathForSession(date, sessionID string) string {
	date = strings.TrimSpace(date)
	if len(date) >= 10 {
		date = date[:10]
	}
	seg := sanitizeDirName(strings.TrimSpace(sessionID))
	if seg == "" {
		return l.DialogHistoryPath(date)
	}
	return filepath.Join(l.Project, date, seg, "dialog_history.json")
}

type AgentScope int

const (
	AgentScopeUser AgentScope = iota
	AgentScopeProject
)

// Layout holds resolved memory directories for a session cwd.
type Layout struct {
	CWD            string
	MemoryBase     string
	User           string
	Project        string
	Auto           string
	TeamUser       string
	TeamProject    string
	AgentDefault   []string // user + project agent memory roots for "default"
	EntrypointName string
}

// DefaultLayout builds standard paths for cwd and home directory.
func DefaultLayout(cwd, home string) Layout {
	mb := MemoryBaseDir(home)
	auto := AutoMemoryDir(cwd, mb)
	return Layout{
		CWD:            cwd,
		MemoryBase:     mb,
		User:           UserMemoryDir(mb),
		Project:        ProjectMemoryDir(cwd),
		Auto:           auto,
		TeamUser:       TeamMemoryDirUser(mb),
		TeamProject:    TeamMemoryDirProject(cwd),
		AgentDefault:   agentDefaultPair(cwd, mb, "default"),
		EntrypointName: entrypointName,
	}
}

func agentDefaultPair(cwd, memoryBase, agentType string) []string {
	return []string{
		AgentMemoryDir(cwd, memoryBase, agentType, AgentScopeUser),
		AgentMemoryDir(cwd, memoryBase, agentType, AgentScopeProject),
	}
}

// AuditWriteRoots is WriteRoots plus behavior-policy directories (for D2 audit coverage).
// Individual AGENT.md files are handled by IsBehaviorPolicyFile.
func (l Layout) AuditWriteRoots() []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(p string) {
		p = filepath.Clean(p)
		if p == "" || p == "." {
			return
		}
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	for _, p := range l.WriteRoots() {
		add(p)
	}
	add(filepath.Join(l.CWD, DotDir, "rules"))
	add(filepath.Join(l.MemoryBase, "rules"))
	add(filepath.Join(l.CWD, DotDir, "skills"))
	add(filepath.Join(l.MemoryBase, "skills"))
	return out
}

// IsBehaviorPolicyFile reports whether abs is one of the canonical AGENT.md locations
// (project `.oneclaw/AGENT.md` or user memory base `~/.oneclaw/AGENT.md`).
func (l Layout) IsBehaviorPolicyFile(abs string) bool {
	abs = filepath.Clean(abs)
	candidates := []string{
		filepath.Join(l.CWD, DotDir, AgentInstructionsFile),
		filepath.Join(l.MemoryBase, AgentInstructionsFile),
	}
	for _, c := range candidates {
		if abs == filepath.Clean(c) {
			return true
		}
	}
	return false
}

// WriteRoots returns distinct directory roots tools may write for memory topics and logs.
func (l Layout) WriteRoots() []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(p string) {
		p = filepath.Clean(p)
		if _, ok := seen[p]; ok {
			return
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	add(l.User)
	add(l.Project)
	if !AutoMemoryDisabled() {
		add(l.Auto)
		add(filepath.Join(l.Auto, "logs"))
	}
	add(l.TeamUser)
	add(l.TeamProject)
	for _, a := range l.AgentDefault {
		add(a)
	}
	return out
}

// defaultAgentMdStub is written when `<cwd>/.oneclaw/AGENT.md` is missing and there is no legacy root AGENT.md to migrate.
const defaultAgentMdStub = `# Agent instructions

Durable behavior rules for this agent in this repository. Edit freely.

- Prefer accurate, tool-grounded answers; avoid guessing when data is missing.

(This file was created automatically because .oneclaw/AGENT.md did not exist.)
`

// EnsureDefaultAgentMd ensures `<cwd>/.oneclaw/AGENT.md` exists: migrates legacy root `AGENT.md` into `.oneclaw/` when the latter is missing, otherwise writes a stub.
func EnsureDefaultAgentMd(l Layout) {
	if l.CWD == "" {
		return
	}
	dotDir := filepath.Join(l.CWD, DotDir)
	dot := filepath.Join(dotDir, AgentInstructionsFile)
	if st, err := os.Stat(dot); err == nil && !st.IsDir() {
		return
	}
	root := filepath.Join(l.CWD, AgentInstructionsFile)
	if err := os.MkdirAll(dotDir, 0o755); err != nil {
		slog.Warn("memory.agent_md.mkdir", "path", dotDir, "err", err)
		return
	}
	if st, err := os.Stat(root); err == nil && !st.IsDir() {
		raw, rerr := os.ReadFile(root)
		if rerr != nil {
			slog.Warn("memory.agent_md.migrate_read", "path", root, "err", rerr)
			return
		}
		if err := os.WriteFile(dot, raw, 0o644); err != nil {
			slog.Warn("memory.agent_md.migrate_write", "path", dot, "err", err)
			return
		}
		slog.Info("memory.agent_md.migrated_from_root", "from", root, "to", dot)
		return
	}
	if err := os.WriteFile(dot, []byte(defaultAgentMdStub), 0o644); err != nil {
		slog.Warn("memory.agent_md.write", "path", dot, "err", err)
		return
	}
	slog.Info("memory.agent_md.created", "path", dot)
}

// EnsureDirs creates memory directories so Write can succeed without mkdir in the model.
func (l Layout) EnsureDirs() {
	for _, d := range l.WriteRoots() {
		_ = os.MkdirAll(d, 0o755)
	}
	EnsureDefaultAgentMd(l)
}

// AutoMemoryDisabled reports features.disable_auto_memory from config.
func AutoMemoryDisabled() bool {
	return rtopts.Current().DisableAutoMemory
}
