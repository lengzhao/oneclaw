// Package memory implements file-based memory discovery, injection, and helpers for phase B.
package memory

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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

func recallSQLitePath(layout Layout) string {
	p := strings.TrimSpace(rtopts.Current().MemoryRecallSQLitePath)
	base := layout.MemoryBase
	if p == "" {
		return filepath.Join(base, "memory", "recall_index.sqlite")
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(base, p)
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
	// InstructionRoot is the IM directory containing AGENT.md and MEMORY.md (same dir). Empty for repo-style layouts.
	InstructionRoot string
	// HostUserData is true when CWD is the IM user data root (~/.oneclaw): config, AGENT.md, audit, and
	// scheduled_maintain_state live directly under CWD, not under CWD/.oneclaw/.
	HostUserData bool
}

// DotOrDataRoot returns the directory holding host-style dot files: for a repo cwd layout it is
// <cwd>/.oneclaw; for IM host layout with InstructionRoot set it is InstructionRoot; otherwise legacy IM host used CWD.
func (l Layout) DotOrDataRoot() string {
	if l.InstructionRoot != "" {
		return filepath.Clean(l.InstructionRoot)
	}
	if l.HostUserData {
		return filepath.Clean(l.CWD)
	}
	return filepath.Join(l.CWD, DotDir)
}

// EpisodeDailyPath returns the episodic digest markdown path for the given calendar day (YYYY-MM-DD prefix).
func (l Layout) EpisodeDailyPath(dateYYYYMMDD string) string {
	dateYYYYMMDD = strings.TrimSpace(dateYYYYMMDD)
	if len(dateYYYYMMDD) >= 10 {
		dateYYYYMMDD = dateYYYYMMDD[:10]
	}
	return filepath.Join(l.Project, dateYYYYMMDD+".md")
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

// SessionDotLayout is for legacy per-session layouts where Engine.CWD was the
// session dot directory (flat files under that directory, no nested ".oneclaw" segment).
func SessionDotLayout(dotRoot, home string) Layout {
	mb := MemoryBaseDir(home)
	dot := filepath.Clean(dotRoot)
	agentProj := filepath.Join(dot, "agent-memory", "default")
	return Layout{
		CWD:            dot,
		MemoryBase:     mb,
		User:           UserMemoryDir(mb),
		Project:        filepath.Join(dot, "memory"),
		Auto:           AutoMemoryDir(dot, mb),
		TeamUser:       TeamMemoryDirUser(mb),
		TeamProject:    filepath.Join(dot, "team-memory"),
		AgentDefault:   []string{filepath.Join(mb, "agent-memory", "default"), agentProj},
		EntrypointName: entrypointName,
		HostUserData:   true,
	}
}

// IMSessionLayout is for IM per-session InstructionRoot (~/.oneclaw/sessions/<id>/): CWD is <InstructionRoot>/workspace.
func IMSessionLayout(instructionRoot, home string) Layout {
	mb := MemoryBaseDir(home)
	ir := filepath.Clean(instructionRoot)
	ws := filepath.Join(ir, IMWorkspaceDirName)
	projMem := filepath.Join(ir, "memory")
	agentProj := filepath.Join(ir, "agent-memory", "default")
	return Layout{
		CWD:             ws,
		InstructionRoot: ir,
		MemoryBase:      mb,
		User:            UserMemoryDir(mb),
		Project:         projMem,
		Auto:            AutoMemoryDir(ws, mb),
		TeamUser:        TeamMemoryDirUser(mb),
		TeamProject:     filepath.Join(ir, "team-memory"),
		AgentDefault:    []string{filepath.Join(mb, "agent-memory", "default"), agentProj},
		EntrypointName:  entrypointName,
		HostUserData:    true,
	}
}

// LayoutForIMWorkspace selects memory layout for an Engine: repo-style DefaultLayout when WorkspaceFlat is false;
// when true, IM shared root uses IMHostMaintainLayout, IMSessionLayout for isolated sessions, or SessionDotLayout (legacy .oneclaw session dir).
func LayoutForIMWorkspace(cwd, home, userDataRoot string, workspaceFlat bool, instructionRoot string) Layout {
	if !workspaceFlat {
		return DefaultLayout(cwd, home)
	}
	// filepath.Clean("") is "." in Go; treat trimmed empty roots as unset.
	urRaw := strings.TrimSpace(userDataRoot)
	var ur string
	if urRaw != "" {
		ur = filepath.Clean(urRaw)
	}
	irRaw := strings.TrimSpace(instructionRoot)
	var ir string
	if irRaw != "" {
		ir = filepath.Clean(irRaw)
	}
	cleanCWD := filepath.Clean(cwd)
	if ir != "" {
		if ur != "" && ir == ur {
			return IMHostMaintainLayout(ur, home)
		}
		return IMSessionLayout(ir, home)
	}
	if ur != "" && cleanCWD == ur {
		return IMHostMaintainLayout(ur, home)
	}
	return SessionDotLayout(cleanCWD, home)
}

// IMHostMaintainLayout is for cmd/oneclaw IM mode: userDataRoot is config.UserDataRoot() (~/.oneclaw).
// CWD is <userDataRoot>/workspace; episodic project memory under <userDataRoot>/memory (no nested .oneclaw).
func IMHostMaintainLayout(userDataRoot, home string) Layout {
	mb := MemoryBaseDir(home)
	ur := filepath.Clean(userDataRoot)
	if ur == "" {
		ur = mb
	}
	agentHost := filepath.Join(ur, "agent-memory", "default")
	ws := filepath.Join(ur, IMWorkspaceDirName)
	projMem := filepath.Join(ur, "memory")
	return Layout{
		CWD:             ws,
		InstructionRoot: ur,
		MemoryBase:      mb,
		User:            UserMemoryDir(mb),
		Project:         projMem,
		Auto:            AutoMemoryDir(ws, mb),
		TeamUser:        TeamMemoryDirUser(mb),
		TeamProject:     filepath.Join(ur, "team-memory"),
		AgentDefault:    []string{filepath.Join(mb, "agent-memory", "default"), agentHost},
		EntrypointName:  entrypointName,
		HostUserData:    true,
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
	add(filepath.Join(l.DotOrDataRoot(), "rules"))
	add(filepath.Join(l.MemoryBase, "rules"))
	add(filepath.Join(l.DotOrDataRoot(), "skills"))
	add(filepath.Join(l.MemoryBase, "skills"))
	return out
}

// IsBehaviorPolicyFile reports whether abs is one of the canonical AGENT.md locations
// (project `.oneclaw/AGENT.md` or user memory base `~/.oneclaw/AGENT.md`).
func (l Layout) IsBehaviorPolicyFile(abs string) bool {
	abs = filepath.Clean(abs)
	candidates := []string{
		filepath.Join(l.DotOrDataRoot(), AgentInstructionsFile),
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

// EnsureDirs creates memory directories so Write can succeed without mkdir in the model.
func (l Layout) EnsureDirs() {
	for _, d := range l.WriteRoots() {
		_ = os.MkdirAll(d, 0o755)
	}
}

// AutoMemoryDisabled reports features.disable_auto_memory from config.
func AutoMemoryDisabled() bool {
	return rtopts.Current().DisableAutoMemory
}
