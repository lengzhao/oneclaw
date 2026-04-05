package memory

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/prompts"
)

// MaintainPromptData fills prompts/templates/maintenance_system.tmpl for memory distillation.
type MaintainPromptData struct {
	CWD        string // project working directory
	Today      string // YYYY-MM-DD, same as digest day
	MemoryPath string // path to project MEMORY.md
	RunTS      string // RFC3339 UTC, wall time of this maintain pass
}

func maintenanceSystemPrompt(cwd, memoryPath, today, runTS string) string {
	d := MaintainPromptData{CWD: cwd, Today: today, MemoryPath: memoryPath, RunTS: runTS}
	s, err := prompts.Render(prompts.NameMaintenanceSystem, d)
	if err != nil {
		slog.Error("memory.prompts.maintenance_system", "err", err)
		return fallbackMaintenanceSystemPrompt(cwd, memoryPath, today, runTS)
	}
	return s
}

func fallbackMaintenanceSystemPrompt(cwd, memoryPath, today, runTS string) string {
	return "You are a silent memory indexer for a coding agent. Scope: project `" + cwd + "`, calendar date " + today + ", target file `" + memoryPath + "`.\n" +
		"Maintenance run started (UTC): " + runTS + ".\n" +
		"Follow the user message format exactly.\n" +
		"Output only the requested markdown section (header + bullets). No preamble or explanation.\n"
}

func appendMaintenanceSection(layout Layout, memPath, section string) error {
	projectDir := layout.Project
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return err
	}
	section = strings.TrimSpace(section)
	var b strings.Builder
	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		b.WriteString("# MEMORY\n\n")
		b.WriteString(section)
		b.WriteString("\n")
		body := b.String()
		if err := os.WriteFile(memPath, []byte(body), 0o644); err != nil {
			return err
		}
		AppendMemoryAudit(layout.CWD, layout.WriteRoots(), memPath, "maintain", []byte(section))
		return nil
	}
	raw, err := os.ReadFile(memPath)
	if err != nil {
		return err
	}
	b.Write(raw)
	if len(raw) > 0 && raw[len(raw)-1] != '\n' {
		b.WriteByte('\n')
	}
	b.WriteString("\n")
	b.WriteString(section)
	b.WriteString("\n")
	body := b.String()
	if err := os.WriteFile(memPath, []byte(body), 0o644); err != nil {
		return err
	}
	AppendMemoryAudit(layout.CWD, layout.WriteRoots(), memPath, "maintain", []byte(section))
	return nil
}

// autoMaintenanceEnabled: on by default; set ONCLAW_DISABLE_AUTO_MAINTENANCE=1/true/yes to turn off.
func autoMaintenanceEnabled() bool {
	v := strings.TrimSpace(os.Getenv("ONCLAW_DISABLE_AUTO_MAINTENANCE"))
	if v == "1" || strings.EqualFold(v, "true") || strings.EqualFold(v, "yes") {
		return false
	}
	return true
}

func maintenanceMinLogBytes() int {
	return getenvIntMaint("ONCLAW_MAINTENANCE_MIN_LOG_BYTES", 200)
}

func maintenanceMaxLogRead() int {
	return getenvIntMaint("ONCLAW_MAINTENANCE_MAX_LOG_BYTES", 24_000)
}

func maintenanceMaxCombinedLogBytes() int {
	n := getenvIntMaint("ONCLAW_MAINTENANCE_MAX_COMBINED_LOG_BYTES", 48_000)
	if n < 1024 {
		n = 1024
	}
	if n > 256_000 {
		n = 256_000
	}
	return n
}

func getenvIntMaint(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	for _, p := range []string{"```markdown", "```md"} {
		if strings.HasPrefix(s, p) {
			s = strings.TrimSpace(s[len(p):])
			break
		}
	}
	if strings.HasPrefix(s, "```") {
		s = strings.TrimSpace(s[3:])
	}
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func utf8SafePrefix(s string, maxBytes int) string {
	if len(s) <= maxBytes {
		return s
	}
	s = s[:maxBytes]
	for !utf8.ValidString(s) {
		if len(s) == 0 {
			return ""
		}
		s = s[:len(s)-1]
	}
	return s
}
