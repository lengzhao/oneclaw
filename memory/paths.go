// Package memory implements file-based memory discovery, injection, and helpers for phase B.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const entrypointName = "MEMORY.md"

// MemoryBaseDir resolves the base config/memory directory (~/.oneclaw or remote override).
func MemoryBaseDir(home string) string {
	if v := strings.TrimSpace(os.Getenv("CLAUDE_CODE_REMOTE_MEMORY_DIR")); v != "" {
		return filepath.Clean(v)
	}
	if v := strings.TrimSpace(os.Getenv("ONCLAW_MEMORY_BASE")); v != "" {
		return filepath.Clean(v)
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

// LocalMemoryDir returns <cwd>/.oneclaw/memory-local.
func LocalMemoryDir(cwd string) string {
	return filepath.Join(cwd, DotDir, "memory-local")
}

// AgentMemoryDir returns the on-disk directory for an agent type and scope.
func AgentMemoryDir(cwd, memoryBase, agentType string, scope AgentScope) string {
	dir := sanitizeDirName(agentType)
	switch scope {
	case AgentScopeUser:
		return filepath.Join(memoryBase, "agent-memory", dir)
	case AgentScopeProject:
		return filepath.Join(cwd, DotDir, "agent-memory", dir)
	case AgentScopeLocal:
		return filepath.Join(cwd, DotDir, "agent-memory-local", dir)
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

// AgentScope selects where agent-scoped memory lives.
type AgentScope int

const (
	AgentScopeUser AgentScope = iota
	AgentScopeProject
	AgentScopeLocal
)

// Layout holds resolved memory directories for a session cwd.
type Layout struct {
	CWD            string
	MemoryBase     string
	User           string
	Project        string
	Local          string
	Auto           string
	TeamUser       string
	TeamProject    string
	AgentDefault   []string // user, project, local roots for agent "default"
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
		Local:          LocalMemoryDir(cwd),
		Auto:           auto,
		TeamUser:       TeamMemoryDirUser(mb),
		TeamProject:    TeamMemoryDirProject(cwd),
		AgentDefault:   agentDefaultTriad(cwd, mb, "default"),
		EntrypointName: entrypointName,
	}
}

func agentDefaultTriad(cwd, memoryBase, agentType string) []string {
	return []string{
		AgentMemoryDir(cwd, memoryBase, agentType, AgentScopeUser),
		AgentMemoryDir(cwd, memoryBase, agentType, AgentScopeProject),
		AgentMemoryDir(cwd, memoryBase, agentType, AgentScopeLocal),
	}
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
	add(l.Local)
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

// EnsureDirs creates memory directories so Write can succeed without mkdir in the model.
func (l Layout) EnsureDirs() {
	for _, d := range l.WriteRoots() {
		_ = os.MkdirAll(d, 0o755)
	}
}

// AutoMemoryDisabled is true only when an explicit opt-out env is set (default: auto memory on).
func AutoMemoryDisabled() bool {
	if v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_AUTO_MEMORY")); v != "" {
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}
	if v := strings.TrimSpace(os.Getenv("CLAUDE_CODE_DISABLE_AUTO_MEMORY")); v != "" {
		return v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes")
	}
	return false
}
