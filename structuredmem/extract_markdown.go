package structuredmem

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	lzmem "github.com/lengzhao/memory"
)

// FormatExtractResultMarkdown renders an ExtractResult as markdown suitable for memory/*.md append.
func FormatExtractResultMarkdown(at time.Time, result *lzmem.ExtractResult) string {
	if result == nil {
		return ""
	}
	if len(result.Memories) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("\n\n---\n\n## lengzhao/memory extract (%s UTC)\n\n",
		at.UTC().Format(time.RFC3339)))
	b.WriteString(fmt.Sprintf("- **status**: %s\n", strings.TrimSpace(result.Status)))
	b.WriteString(fmt.Sprintf("- **extraction_id**: `%s`\n", strings.TrimSpace(result.ExtractionID)))
	if result.TotalTokens > 0 {
		b.WriteString(fmt.Sprintf("- **tokens**: %d\n", result.TotalTokens))
	}
	if result.ProcessingTime > 0 {
		b.WriteString(fmt.Sprintf("- **processing_ms**: %d\n", result.ProcessingTime))
	}
	b.WriteString(fmt.Sprintf("\n### Memories (%d)\n\n", len(result.Memories)))
	for i, m := range result.Memories {
		title := strings.TrimSpace(m.Title)
		if title == "" {
			title = "(untitled)"
		}
		b.WriteString(fmt.Sprintf("%d. **[%s]** %s\n", i+1, m.Namespace, title))
		if s := strings.TrimSpace(m.Summary); s != "" {
			b.WriteString(fmt.Sprintf("   - summary: %s\n", s))
		}
		if s := strings.TrimSpace(m.Content); s != "" {
			trim := s
			if len([]rune(trim)) > 500 {
				trim = string([]rune(trim)[:500]) + "…"
			}
			b.WriteString(fmt.Sprintf("   - content: %s\n", trim))
		}
		b.WriteString(fmt.Sprintf("   - confidence: %.3f | importance: %d\n", m.Confidence, m.Importance))
		if len(m.Tags) > 0 {
			b.WriteString(fmt.Sprintf("   - tags: %s\n", strings.Join(m.Tags, ", ")))
		}
	}
	if result.DecisionResult != nil {
		if raw, err := json.MarshalIndent(result.DecisionResult, "", "  "); err == nil {
			b.WriteString("\n<details><summary>decision engine</summary>\n\n```json\n")
			b.Write(raw)
			b.WriteString("\n```\n\n</details>\n")
		}
	}
	return b.String()
}
