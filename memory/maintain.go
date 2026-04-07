package memory

import (
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/prompts"
)

// MaintainPromptData fills maintenance system templates for memory distillation.
type MaintainPromptData struct {
	CWD             string // project working directory
	Today           string // YYYY-MM-DD, same as digest day
	MemoryPath      string // episodic digest file for this calendar day (.oneclaw/memory/YYYY-MM-DD.md)
	RulesMemoryPath string // project MEMORY.md (rules only; excerpt for dedupe)
	RunTS           string // RFC3339 UTC, wall time of this maintain pass
}

// Audit sources for AppendMemoryAudit when appending Auto-maintained sections.
const (
	AuditSourcePostTurnMaintain  = "post_turn_maintain"
	AuditSourceScheduledMaintain = "scheduled_maintain"
)

func maintenanceSystemPromptForPathway(p maintainPathway, cwd, episodePath, rulesPath, today, runTS string) string {
	d := MaintainPromptData{CWD: cwd, Today: today, MemoryPath: episodePath, RulesMemoryPath: rulesPath, RunTS: runTS}
	name := prompts.NameMaintenanceSystemScheduled
	if p == pathwayPostTurn {
		name = prompts.NameMaintenanceSystemPostTurn
	}
	s, err := prompts.Render(name, d)
	if err != nil {
		slog.Error("memory.prompts.maintenance_system", "pathway", p, "err", err)
		return fallbackMaintenanceSystemPrompt(cwd, episodePath, rulesPath, today, runTS, p == pathwayPostTurn)
	}
	return s
}

func fallbackMaintenanceSystemPrompt(cwd, episodePath, rulesPath, today, runTS string, postTurn bool) string {
	kind := "consolidation"
	if postTurn {
		kind = "post-turn"
	}
	scope := "Synthesize across recent logs; episodic digest → `" + episodePath + "`; dedupe using rules in `" + rulesPath + "`."
	if postTurn {
		scope = "Near-field: only this finished user turn (snapshot); episodic digest → `" + episodePath + "`; rules excerpt → `" + rulesPath + "` for dedupe."
	}
	out := "You are a silent memory indexer for a coding agent (" + kind + "). Scope: project `" + cwd + "`, calendar date " + today + ", episodic digest `" + episodePath + "`, rules `" + rulesPath + "`.\n" +
		scope + "\n"
	if !postTurn {
		out += "Session records (optional; read-only tools): per-day slim dialogue JSON `" + cwd + "/.oneclaw/memory/" + today + "/dialog_history.json` (other days: replace date segment); model context `" + cwd + "/.oneclaw/working_transcript.json`; cumulative slim `" + cwd + "/.oneclaw/transcript.json`.\n"
	}
	out += "Maintenance run started (UTC): " + runTS + ".\n" +
		"Follow the user message format exactly.\n" +
		"Output only the requested markdown section (header + bullets). No preamble or explanation.\n"
	return out
}

func isEpisodeDigestFile(layout Layout, memPath string) bool {
	clean := filepath.Clean(memPath)
	projMem := filepath.Clean(ProjectMemoryDir(layout.CWD))
	if !PathUnderRoot(clean, projMem) {
		return false
	}
	rel, err := filepath.Rel(projMem, clean)
	if err != nil || strings.Contains(rel, string(filepath.Separator)) {
		return false
	}
	base := filepath.Base(clean)
	if strings.EqualFold(base, entrypointName) {
		return false
	}
	if !strings.HasSuffix(strings.ToLower(base), ".md") {
		return false
	}
	datePart := strings.TrimSuffix(base, ".md")
	if len(datePart) != 10 {
		return false
	}
	_, err = time.Parse("2006-01-02", datePart)
	return err == nil
}

func appendMaintenanceSection(layout Layout, memPath, section, auditSource string) error {
	dir := filepath.Dir(memPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	section = strings.TrimSpace(section)
	var b strings.Builder
	if _, err := os.Stat(memPath); os.IsNotExist(err) {
		if !isEpisodeDigestFile(layout, memPath) {
			b.WriteString("# MEMORY\n\n")
		}
		b.WriteString(section)
		b.WriteString("\n")
		body := b.String()
		if err := os.WriteFile(memPath, []byte(body), 0o644); err != nil {
			return err
		}
		if auditSource == "" {
			auditSource = AuditSourceScheduledMaintain
		}
		AppendMemoryAudit(layout, memPath, auditSource, []byte(section))
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
	if auditSource == "" {
		auditSource = AuditSourceScheduledMaintain
	}
	AppendMemoryAudit(layout, memPath, auditSource, []byte(section))
	return nil
}

// writeMergedOrAppendMaintenanceSection replaces today's digest span when hadSpan, otherwise appends (new file or new day).
func writeMergedOrAppendMaintenanceSection(layout Layout, memPath string, hadSpan bool, spanStart, spanEnd int, existingFile, mergedSection, auditSource string) error {
	mergedSection = strings.TrimSpace(mergedSection)
	if !hadSpan {
		return appendMaintenanceSection(layout, memPath, mergedSection, auditSource)
	}
	newBody := existingFile[:spanStart] + mergedSection + existingFile[spanEnd:]
	if !strings.HasSuffix(newBody, "\n") {
		newBody += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(memPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(memPath, []byte(newBody), 0o644); err != nil {
		return err
	}
	if auditSource == "" {
		auditSource = AuditSourceScheduledMaintain
	}
	AppendMemoryAudit(layout, memPath, auditSource, []byte(mergedSection))
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

// countMaintenanceBullets counts non-empty lines that start with "- " (markdown bullets) in a maintenance section.
func countMaintenanceBullets(section string) int {
	n := 0
	for _, line := range strings.Split(section, "\n") {
		s := strings.TrimSpace(line)
		if len(s) >= 2 && strings.HasPrefix(s, "- ") {
			n++
		}
	}
	return n
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
