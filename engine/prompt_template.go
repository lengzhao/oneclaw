package engine

import "github.com/lengzhao/oneclaw/session"

// EnsurePromptTemplateData returns the mutable prompt template map, allocating if nil.
func (rtx *RuntimeContext) EnsurePromptTemplateData() map[string]any {
	if rtx == nil {
		return nil
	}
	if rtx.PromptTemplateData == nil {
		rtx.PromptTemplateData = make(map[string]any)
	}
	return rtx.PromptTemplateData
}

// SetPromptTemplateEntry sets one key in PromptTemplateData (no-op on nil rtx or empty key).
func (rtx *RuntimeContext) SetPromptTemplateEntry(key string, value any) {
	if rtx == nil || key == "" {
		return
	}
	rtx.EnsurePromptTemplateData()[key] = value
}

// PromptTemplateRaw returns a stored template value by key.
func (rtx *RuntimeContext) PromptTemplateRaw(key string) (any, bool) {
	if rtx == nil || rtx.PromptTemplateData == nil || key == "" {
		return nil, false
	}
	v, ok := rtx.PromptTemplateData[key]
	return v, ok
}

// PromptTemplateDataCopy returns a shallow copy for template execution (caller may mutate the returned map).
func (rtx *RuntimeContext) PromptTemplateDataCopy() map[string]any {
	out := make(map[string]any)
	if rtx == nil || rtx.PromptTemplateData == nil {
		return out
	}
	for k, v := range rtx.PromptTemplateData {
		out[k] = v
	}
	return out
}

// SetTranscriptReplayTurns replaces transcript replay messages for adk_main (load_transcript node).
func (rtx *RuntimeContext) SetTranscriptReplayTurns(turns []session.TranscriptTurn) {
	if rtx == nil {
		return
	}
	rtx.TranscriptReplayTurns = turns
}
