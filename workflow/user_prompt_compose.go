package workflow

import (
	"fmt"
	"strings"
)

// ComposeUserPrompt returns base with optional prefix/suffix from workflow meta.
// Keys (optional, trimmed):
//   - user_prompt_prefix — inserted before base (after trim), separated by a blank line when base non-empty
//   - user_prompt_suffix — appended after base, separated by a blank line when base non-empty
//
// Memory recall and $start.user_prompt resolution use the raw base only; the executor applies this
// composition when building ADK messages and run journal user_message.
func ComposeUserPrompt(meta map[string]any, base string) string {
	base = strings.TrimSpace(base)
	pre := metaPromptAffix(meta, "user_prompt_prefix")
	suf := metaPromptAffix(meta, "user_prompt_suffix")
	if pre == "" && suf == "" {
		return base
	}
	var b strings.Builder
	if pre != "" {
		b.WriteString(pre)
		if base != "" || suf != "" {
			b.WriteString("\n\n")
		}
	}
	b.WriteString(base)
	if suf != "" {
		if pre != "" || base != "" {
			b.WriteString("\n\n")
		}
		b.WriteString(suf)
	}
	return strings.TrimSpace(b.String())
}

func metaPromptAffix(meta map[string]any, key string) string {
	if len(meta) == 0 {
		return ""
	}
	v, ok := meta[key]
	if !ok || v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}
