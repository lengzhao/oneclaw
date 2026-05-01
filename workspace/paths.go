// Package workspace holds user/session path layout, write roots, and related helpers (no LLM recall or maintenance).
package workspace

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/rtopts"
)

// DotDir is the default user data root directory name under HOME.
const DotDir = ".oneclaw"

// AgentInstructionsFile is the repo/user instructions entry filename.
const AgentInstructionsFile = "AGENT.md"

// RulesMemoryFile is the short rules / standing memory entry (same directory as AGENT.md in IM layouts).
const RulesMemoryFile = "MEMORY.md"

// SoulFile is optional persona / tone notes (same directory as AGENT.md when used).
const SoulFile = "SOUL.md"

// TodoFile is optional freeform session todo notes (same directory as AGENT.md when used).
const TodoFile = "TODO.md"

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

// ProjectMemoryDir returns <cwd>/memory.
func ProjectMemoryDir(cwd string) string {
	return filepath.Join(cwd, "memory")
}

// AgentMemoryDir returns the on-disk directory for an agent type and scope.
func AgentMemoryDir(cwd, memoryBase, agentType string, scope AgentScope) string {
	dir := sanitizeDirName(agentType)
	switch scope {
	case AgentScopeUser:
		return filepath.Join(memoryBase, "agent-memory", dir)
	case AgentScopeProject:
		return filepath.Join(cwd, "agent-memory", dir)
	default:
		return filepath.Join(cwd, "agent-memory", dir)
	}
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

// DialogHistoryPath returns <project-memory>/YYYY-MM-DD/dialog_history.json (calendar date, local).
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

// PathUnderRoot reports whether abs is root or a path strictly under root (after filepath.Clean).
func PathUnderRoot(abs, root string) bool {
	abs = filepath.Clean(abs)
	root = filepath.Clean(root)
	if root == "" || root == "." {
		return false
	}
	if abs == root {
		return true
	}
	rel, err := filepath.Rel(root, abs)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

type AgentScope int

const (
	AgentScopeUser AgentScope = iota
	AgentScopeProject
)

// Layout holds resolved workspace directories for a session cwd.
type Layout struct {
	CWD            string
	MemoryBase     string
	User           string
	Project        string
	Auto           string
	AgentDefault   []string // user + project agent memory roots for "default"
	EntrypointName string
	// InstructionRoot is the IM directory containing AGENT.md and MEMORY.md (same dir). Empty for repo-style layouts.
	InstructionRoot string
	// HostUserData is true when CWD is the IM user data root (~/.oneclaw): config, AGENT.md, and
	// other host-level runtime files live directly under CWD.
	HostUserData bool
}

// DotOrDataRoot returns the directory holding host-style instruction/runtime files:
// IM InstructionRoot when set; else repo <cwd>/.oneclaw when present; else legacy host CWD or plain repo CWD.
func (l Layout) DotOrDataRoot() string {
	if l.InstructionRoot != "" {
		return filepath.Clean(l.InstructionRoot)
	}
	if l.HostUserData {
		return filepath.Clean(l.CWD)
	}
	if ov := l.RepoOverlayDir(); ov != "" {
		return ov
	}
	return filepath.Clean(l.CWD)
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
		AgentDefault:   agentDefaultPair(cwd, mb, "default"),
		EntrypointName: RulesMemoryFile,
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
		AgentDefault:   []string{filepath.Join(mb, "agent-memory", "default"), agentProj},
		EntrypointName: RulesMemoryFile,
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
		AgentDefault:    []string{filepath.Join(mb, "agent-memory", "default"), agentProj},
		EntrypointName:  RulesMemoryFile,
		HostUserData:    true,
	}
}

// LayoutForIMWorkspace selects layout for an Engine: repo-style DefaultLayout when WorkspaceFlat is false;
// when true, IM shared root uses IMHostMaintainLayout, IMSessionLayout for isolated sessions, or SessionDotLayout (legacy session dir).
func LayoutForIMWorkspace(cwd, home, userDataRoot string, workspaceFlat bool, instructionRoot string) Layout {
	if !workspaceFlat {
		return DefaultLayout(cwd, home)
	}
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
		AgentDefault:    []string{filepath.Join(mb, "agent-memory", "default"), agentHost},
		EntrypointName:  RulesMemoryFile,
		HostUserData:    true,
	}
}

func agentDefaultPair(cwd, memoryBase, agentType string) []string {
	return []string{
		AgentMemoryDir(cwd, memoryBase, agentType, AgentScopeUser),
		AgentMemoryDir(cwd, memoryBase, agentType, AgentScopeProject),
	}
}

// WriteRoots returns distinct directory roots tools may write for topics and logs.
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
	for _, a := range l.AgentDefault {
		add(a)
	}
	return out
}

// EnsureDirs creates workspace directories so Write can succeed without mkdir in the model.
func (l Layout) EnsureDirs() {
	for _, d := range l.WriteRoots() {
		_ = os.MkdirAll(d, 0o755)
	}
}

// AutoMemoryDisabled reports features.disable_auto_memory from config.
func AutoMemoryDisabled() bool {
	return rtopts.Current().DisableAutoMemory
}

// RepoOverlayDir returns <cwd>/.oneclaw when it exists as a directory (repo-style overlay); empty otherwise.
// Not used when InstructionRoot is set (IM layouts).
func (l Layout) RepoOverlayDir() string {
	if strings.TrimSpace(l.InstructionRoot) != "" {
		return ""
	}
	dot := filepath.Join(filepath.Clean(l.CWD), DotDir)
	if st, err := os.Stat(dot); err == nil && st.IsDir() {
		return dot
	}
	return ""
}

// RulesEntryDir is the directory holding project MEMORY.md / SOUL.md / TODO.md beside AGENT.md:
// IM InstructionRoot when set, else repo overlay when present, else the project memory root directory.
func (l Layout) RulesEntryDir() string {
	if ir := strings.TrimSpace(l.InstructionRoot); ir != "" {
		return filepath.Clean(ir)
	}
	if ov := l.RepoOverlayDir(); ov != "" {
		return ov
	}
	return filepath.Clean(l.Project)
}
