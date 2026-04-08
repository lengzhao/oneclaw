package notify

import "github.com/lengzhao/oneclaw/loop"

// ToolCallEndData maps loop.ToolTraceEntry into notify Data (MVP).
func ToolCallEndData(ent loop.ToolTraceEntry) map[string]any {
	m := map[string]any{
		"model_step":   ent.Step,
		"name":         ent.Name,
		"ok":           ent.OK,
		"duration_ms":  ent.DurationMs,
		"args_preview": ent.ArgsPreview,
		"out_preview":  ent.OutPreview,
	}
	if ent.ToolUseID != "" {
		m["tool_use_id"] = ent.ToolUseID
	}
	if ent.Err != "" {
		m["err"] = ent.Err
	}
	return m
}
