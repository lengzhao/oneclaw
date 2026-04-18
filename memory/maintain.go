package memory

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
	"unicode/utf8"

	"github.com/lengzhao/oneclaw/prompts"
	"github.com/lengzhao/oneclaw/rtopts"
)

// Optional user-authored maintenance system prompts (same directory as AGENT.md / MEMORY.md: [Layout.DotOrDataRoot]).
// Content is rendered with [text/template] and [MaintainPromptData] (e.g. {{.CWD}}, {{.MemoryPath}}, {{.Today}}).
// Copy built-in wording from prompts/templates/maintenance_system_*.tmpl in this repo as a starting point.
const (
	MaintainScheduledSystemFile = "MAINTAIN_SCHEDULED.md"
	MaintainPostTurnSystemFile  = "MAINTAIN_POST_TURN.md"
)

// MaintainPromptData fills maintenance system templates for memory distillation.
type MaintainPromptData struct {
	CWD                   string // workspace / data root label for prompts
	Today                 string // YYYY-MM-DD, same as digest day
	MemoryPath            string // episodic digest file for this calendar day
	RulesMemoryPath       string // project MEMORY.md (rules only; excerpt for dedupe)
	RunTS                 string // RFC3339 UTC, wall time of this maintain pass
	DialogHistoryPath     string // scheduled: today's dialog_history.json (absolute)
	WorkingTranscriptPath string // scheduled: working_transcript.json (absolute)
	TranscriptPath        string // scheduled: transcript.json (absolute)
}

// Audit sources for AppendMemoryAudit when appending Auto-maintained sections.
const (
	AuditSourcePostTurnMaintain  = "post_turn_maintain"
	AuditSourceScheduledMaintain = "scheduled_maintain"
)

func maintenanceSystemPromptForPathway(p maintainPathway, layout Layout, episodePath, rulesPath, today, runTS string) string {
	root := layout.DotOrDataRoot()
	d := MaintainPromptData{
		CWD:                   layout.CWD,
		Today:                 today,
		MemoryPath:            episodePath,
		RulesMemoryPath:       rulesPath,
		RunTS:                 runTS,
		DialogHistoryPath:     filepath.Clean(layout.DialogHistoryPath(today)),
		WorkingTranscriptPath: filepath.Clean(filepath.Join(root, "working_transcript.json")),
		TranscriptPath:        filepath.Clean(filepath.Join(root, "transcript.json")),
	}
	if custom, ok := userMaintenanceSystemPrompt(root, p, d); ok {
		return custom
	}
	name := prompts.NameMaintenanceSystemScheduled
	if p == pathwayPostTurn {
		name = prompts.NameMaintenanceSystemPostTurn
	}
	s, err := prompts.Render(name, d)
	if err != nil {
		slog.Error("memory.prompts.maintenance_system", "pathway", p, "err", err)
		return fallbackMaintenanceSystemPrompt(layout, episodePath, rulesPath, today, runTS, p == pathwayPostTurn)
	}
	return s
}

// userMaintenanceSystemPrompt loads MAINTAIN_SCHEDULED.md / MAINTAIN_POST_TURN.md under root when present.
func userMaintenanceSystemPrompt(root string, p maintainPathway, d MaintainPromptData) (string, bool) {
	var fname string
	switch p {
	case pathwayPostTurn:
		fname = MaintainPostTurnSystemFile
	case pathwayScheduled:
		fname = MaintainScheduledSystemFile
	default:
		return "", false
	}
	path := filepath.Join(root, fname)
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	raw := strings.TrimSpace(string(b))
	if raw == "" {
		return "", false
	}
	out, err := renderMaintainUserSystemTemplate(raw, d)
	if err != nil {
		slog.Warn("memory.maintain.user_system_template", "path", path, "err", err)
		return "", false
	}
	out = strings.TrimSpace(out)
	if out == "" {
		return "", false
	}
	return out, true
}

func renderMaintainUserSystemTemplate(content string, d MaintainPromptData) (string, error) {
	t, err := template.New("maintain_user").Parse(content)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func fallbackMaintenanceSystemPrompt(layout Layout, episodePath, rulesPath, today, runTS string, postTurn bool) string {
	kind := "consolidation"
	if postTurn {
		kind = "post-turn"
	}
	scope := "Synthesize across recent logs; episodic digest → `" + episodePath + "`; dedupe using rules in `" + rulesPath + "`."
	if postTurn {
		scope = "Near-field: only this finished user turn (snapshot); episodic digest → `" + episodePath + "`; rules excerpt → `" + rulesPath + "` for dedupe."
	}
	dialog := filepath.Clean(layout.DialogHistoryPath(today))
	root := layout.DotOrDataRoot()
	workT := filepath.Clean(filepath.Join(root, "working_transcript.json"))
	slimT := filepath.Clean(filepath.Join(root, "transcript.json"))
	out := "You are a silent memory indexer for a coding agent (" + kind + "). Scope: project `" + layout.CWD + "`, calendar date " + today + ", episodic digest `" + episodePath + "`, rules `" + rulesPath + "`.\n" +
		scope + "\n"
	if !postTurn {
		out += "Session records (optional; read-only tools): per-day slim dialogue JSON `" + dialog + "` (other days: replace date segment); model context `" + workT + "`; cumulative slim `" + slimT + "`.\n"
	}
	out += "Maintenance run started (UTC): " + runTS + ".\n" +
		"Follow the user message format exactly.\n" +
		"Output only the requested markdown section (header + bullets). No preamble or explanation.\n"
	return out
}

func isEpisodeDigestFile(layout Layout, memPath string) bool {
	clean := filepath.Clean(memPath)
	projMem := filepath.Clean(layout.Project)
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

func autoMaintenanceEnabled() bool {
	return !rtopts.Current().DisableAutoMaintenance
}

func maintenanceMinLogBytes() int {
	return rtopts.Current().MaintenanceMinLogBytes
}

func maintenanceMaxLogRead() int {
	return rtopts.Current().MaintenanceMaxLogRead
}

func maintenanceMaxCombinedLogBytes() int {
	return rtopts.Current().MaintenanceMaxCombinedLogBytes
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
