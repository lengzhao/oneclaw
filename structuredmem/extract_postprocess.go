package structuredmem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	lzmem "github.com/lengzhao/memory"

	mempaths "github.com/lengzhao/oneclaw/memory"
)

// ApplyExtractPostprocess filters noisy memories for lengzhao/memory SQLite persistence and journals MD append,
// and writes human-readable superseded notes when conflicting facts appear in the same extract batch.
func ApplyExtractPostprocess(instructionRoot string, at time.Time, result *lzmem.ExtractResult) {
	if result == nil || len(result.Memories) == 0 {
		return
	}
	filtered := filterExtractedMemories(result.Memories)
	conflicts := detectPMOAIInfraMeetingConflicts(filtered)
	filtered = applyPMOAIInfraMeetingConflictResolution(filtered, conflicts)
	result.Memories = filtered

	if len(conflicts) == 0 {
		return
	}
	md := FormatConflictingMemoriesMarkdown(at, conflicts)
	if strings.TrimSpace(md) == "" {
		return
	}
	rel, err := mempaths.NormalizeMemoryMonthRel("")
	if err != nil {
		return
	}
	abs, err := mempaths.ResolveMemoryMonthMarkdown(instructionRoot, rel)
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(abs, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = f.WriteString(md)
}

func filterExtractedMemories(in []lzmem.ExtractedMemory) []lzmem.ExtractedMemory {
	out := make([]lzmem.ExtractedMemory, 0, len(in))
	for _, m := range in {
		if shouldDropExtractedMemory(m) {
			continue
		}
		out = append(out, m)
	}
	return out
}

func shouldDropExtractedMemory(m lzmem.ExtractedMemory) bool {
	ns := strings.ToLower(strings.TrimSpace(string(m.Namespace)))
	switch ns {
	case string(lzmem.NamespaceTransient):
		return shouldDropTransientMemory(m)
	default:
		return false
	}
}

func shouldDropTransientMemory(m lzmem.ExtractedMemory) bool {
	text := normalizeMemoryText(m.Title, m.Summary, m.Content)
	if text == "" {
		return true
	}
	keys := []string{
		"当前时间", "现在时间", "utc", "今天", "现在是", "timestamp", "几点",
		"日期解析", "明天", "对应 ", // ephemeral calendar extrapolation from "tomorrow"
	}
	lt := strings.ToLower(text)
	for _, k := range keys {
		if k == "" {
			continue
		}
		if strings.Contains(lt, strings.ToLower(k)) {
			return true
		}
	}
	return false
}

func normalizeMemoryText(parts ...string) string {
	var b strings.Builder
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(p)
	}
	s := b.String()
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(strings.Join(strings.Fields(s), " "))
}

func memoryMentionsPMOAndAIInfra(m lzmem.ExtractedMemory) bool {
	text := normalizeMemoryText(m.Title, m.Summary, m.Content)
	if text == "" {
		return false
	}
	lt := strings.ToLower(text)
	hasPMO := strings.Contains(lt, "pmo")
	hasAI := strings.Contains(lt, "ai infra") || strings.Contains(lt, "aiinfra") || strings.Contains(lt, "ai-infra")
	return hasPMO && hasAI
}

func memoryImpliesTwoMeetings(text string) bool {
	text = strings.ToLower(text)
	if strings.Contains(text, "两场会议") || strings.Contains(text, "两场 ") || strings.Contains(text, "两条会议") || strings.Contains(text, "两条子任务") {
		return true
	}
	if strings.Contains(text, "两场分开") {
		return true
	}
	if strings.Contains(text, "各自") && strings.Contains(text, "会议") && strings.Contains(text, "两个") {
		return true
	}
	return false
}

func memoryImpliesSingleMeeting(text string) bool {
	text = strings.ToLower(text)
	// Prefer explicit affirmative statements; negations like "不是两场分开" often appear as clarifications
	// alongside count phrases ("两场会议") and are ambiguous without an affirmative cue.
	return strings.Contains(text, "同一场") || strings.Contains(text, "一场会议") || strings.Contains(text, "合并成一个会议") || strings.Contains(text, "合并会议")
}

type memoryConflict struct {
	Superseded lzmem.ExtractedMemory
	Kept       lzmem.ExtractedMemory
	Reason     string
}

func detectPMOAIInfraMeetingConflicts(mem []lzmem.ExtractedMemory) []memoryConflict {
	var singles []lzmem.ExtractedMemory
	var doubles []lzmem.ExtractedMemory
	for _, m := range mem {
		if !memoryMentionsPMOAndAIInfra(m) {
			continue
		}
		text := normalizeMemoryText(m.Title, m.Summary, m.Content)
		if text == "" {
			continue
		}
		switch {
		case memoryImpliesSingleMeeting(text):
			singles = append(singles, m)
		case memoryImpliesTwoMeetings(text):
			doubles = append(doubles, m)
		}
	}
	if len(singles) == 0 || len(doubles) == 0 {
		return nil
	}
	// Pair arbitrarily but deterministically: supersede all "two meetings" memories when any single-meeting fact exists.
	out := make([]memoryConflict, 0, len(doubles))
	keep := singles[len(singles)-1]
	for _, d := range doubles {
		out = append(out, memoryConflict{
			Superseded: d,
			Kept:       keep,
			Reason:     "user clarified PMO + AI Infra are one meeting in the same extract batch",
		})
	}
	return out
}

func applyPMOAIInfraMeetingConflictResolution(mem []lzmem.ExtractedMemory, conflicts []memoryConflict) []lzmem.ExtractedMemory {
	if len(conflicts) == 0 {
		return mem
	}
	drop := make(map[string]struct{}, len(conflicts))
	for _, c := range conflicts {
		drop[extractedMemoryStableKey(c.Superseded)] = struct{}{}
	}
	out := make([]lzmem.ExtractedMemory, 0, len(mem))
	for _, m := range mem {
		if _, ok := drop[extractedMemoryStableKey(m)]; ok {
			continue
		}
		out = append(out, m)
	}
	return out
}

func extractedMemoryStableKey(m lzmem.ExtractedMemory) string {
	return strings.TrimSpace(string(m.Namespace)) + "|" + normalizeMemoryText(m.Title, m.Summary, m.Content)
}

// FormatConflictingMemoriesMarkdown writes an audit fragment explaining superseded memories for the human-readable journal.
func FormatConflictingMemoriesMarkdown(at time.Time, conflicts []memoryConflict) string {
	if len(conflicts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n\n---\n\n")
	b.WriteString(fmt.Sprintf("## structuredmem conflict resolution (%s UTC)\n\n", at.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("Superseded memories (%d):\n\n", len(conflicts)))
	for i, c := range conflicts {
		title := strings.TrimSpace(c.Superseded.Title)
		if title == "" {
			title = "(untitled)"
		}
		b.WriteString(fmt.Sprintf("%d. **[%s]** %s — %s\n", i+1, c.Superseded.Namespace, title, strings.TrimSpace(c.Reason)))
		if s := normalizeMemoryText(c.Superseded.Summary); s != "" {
			b.WriteString("   - superseded_summary: ")
			b.WriteString(s)
			b.WriteString("\n")
		}
		if k := normalizeMemoryText(c.Kept.Title, c.Kept.Summary); k != "" {
			b.WriteString("   - kept: ")
			b.WriteString(k)
			b.WriteString("\n")
		}
	}
	return b.String()
}

func userFacingPreferencePromotionText(m lzmem.ExtractedMemory) string {
	if !strings.EqualFold(strings.TrimSpace(string(m.Namespace)), string(lzmem.NamespaceProfile)) {
		return ""
	}
	text := normalizeMemoryText(m.Title, m.Summary, m.Content)
	if text == "" {
		return ""
	}
	// Promote only explicit user-directed naming preferences; avoid storing assistant self-identification as durable profile memory.
	patterns := []string{
		"叫我", "称呼我", "叫我庆哥", "请叫我", "你可以叫我", "希望叫我", "preferred name", "call me",
	}
	lt := strings.ToLower(text)
	for _, p := range patterns {
		if p == "" {
			continue
		}
		if strings.Contains(text, p) || strings.Contains(lt, strings.ToLower(p)) {
			return text
		}
	}
	return ""
}

func looksLikeAssistantSelfIdentification(m lzmem.ExtractedMemory) bool {
	if !strings.EqualFold(strings.TrimSpace(string(m.Namespace)), string(lzmem.NamespaceProfile)) {
		return false
	}
	text := normalizeMemoryText(m.Title, m.Summary, m.Content)
	if text == "" {
		return false
	}
	lt := strings.ToLower(text)
	if strings.Contains(lt, "assistant") && (strings.Contains(lt, "name") || strings.Contains(lt, "call") || strings.Contains(lt, "称呼")) {
		return true
	}
	if strings.Contains(text, "助手") && (strings.Contains(text, "名字") || strings.Contains(text, "叫")) {
		return true
	}
	// Heuristic: assistant talking about itself in third person with explicit naming quotes.
	if strings.Contains(text, "我叫") && strings.Contains(text, "助手") {
		return true
	}
	return false
}
