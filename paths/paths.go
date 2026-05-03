// Package paths resolves UserDataRoot, SessionRoot, InstructionRoot, and workspace paths (docs/appendix-data-layout.md).
package paths

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lengzhao/oneclaw/config"
)

// EnvUserDataRoot overrides the default ~/.oneclaw when set.
const EnvUserDataRoot = "ONECLAW_USER_DATA_ROOT"

// ResolveUserDataRoot returns the configured or default user data root (expanded).
func ResolveUserDataRoot(f *config.File) (string, error) {
	if f != nil && strings.TrimSpace(f.UserDataRoot) != "" {
		return ExpandHome(strings.TrimSpace(f.UserDataRoot))
	}
	if v := strings.TrimSpace(os.Getenv(EnvUserDataRoot)); v != "" {
		return ExpandHome(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".oneclaw"), nil
}

// SessionRoot is UserDataRoot/sessions/<sessionID>/.
func SessionRoot(userDataRoot, sessionID string) string {
	return filepath.Join(userDataRoot, "sessions", sessionID)
}

// InstructionRoot returns SessionRoot when isolate is true, else UserDataRoot (appendix §2–§3).
func InstructionRoot(userDataRoot, sessionID string, isolate bool) string {
	if isolate {
		return SessionRoot(userDataRoot, sessionID)
	}
	return userDataRoot
}

// Workspace returns <instructionRoot>/workspace.
func Workspace(instructionRoot string) string {
	return filepath.Join(instructionRoot, "workspace")
}

// SubSessionRoot is sessions/<parent>/subs/<sub_run_id>/ (appendix-data-layout §3.1).
func SubSessionRoot(parentSessionRoot, subRunID string) string {
	return filepath.Join(parentSessionRoot, "subs", subRunID)
}

// CatalogRoot is UserDataRoot: manifest.yaml, agents/, skills/, workflows/, prompts/ live here (no hidden subfolder).
func CatalogRoot(userDataRoot string) string {
	return userDataRoot
}

// ScheduledJobsPath is the persisted timer/cron job list (appendix-data-layout §2; reference-architecture §2.6).
func ScheduledJobsPath(userDataRoot string) string {
	return filepath.Join(userDataRoot, "scheduled_jobs.json")
}

// SeedInstructionFiles copies UserDataRoot/AGENT.md and MEMORY.md into InstructionRoot when missing there (session bootstrap).
func SeedInstructionFiles(userDataRoot, instructionRoot string) error {
	pairs := [][2]string{
		{filepath.Join(userDataRoot, "AGENT.md"), filepath.Join(instructionRoot, "AGENT.md")},
		{filepath.Join(userDataRoot, "MEMORY.md"), filepath.Join(instructionRoot, "MEMORY.md")},
	}
	for _, p := range pairs {
		if _, err := os.Stat(p[1]); err == nil {
			continue
		}
		b, err := os.ReadFile(p[0])
		if err != nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p[1]), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p[1], b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// ExpandHome replaces a leading "~" or "~/" with the current user's home directory.
func ExpandHome(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" || p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, p[2:]), nil
	}
	return filepath.Clean(p), nil
}
