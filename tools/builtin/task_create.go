package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/lengzhao/oneclaw/tasks"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type taskCreateInput struct {
	Replace bool `json:"replace"`
	Tasks   []struct {
		ID          string            `json:"id"`
		Subject     string            `json:"subject"`
		Description string            `json:"description"`
		Status      string            `json:"status"`
		Owner       string            `json:"owner"`
		DependsOn   []string          `json:"depends_on"`
		Metadata    map[string]string `json:"metadata"`
	} `json:"tasks"`
}

// TaskCreateTool persists a structured task list for multi-step work (session-scoped, survives restarts).
type TaskCreateTool struct{}

func (TaskCreateTool) Name() string          { return "task_create" }
func (TaskCreateTool) ConcurrencySafe() bool { return false }

func (TaskCreateTool) Description() string {
	return "Create or replace the persisted task list for this session runtime (`tasks.json` under the active workspace root). Use for complex multi-step work so progress survives long sessions and restarts. Prefer replace false to append; use replace true to reset the whole list. Each task needs a clear subject; set status to pending (default), in_progress, completed, or cancelled."
}

func (TaskCreateTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"replace": map[string]any{
			"type":        "boolean",
			"description": "If true, discard existing tasks and use only the tasks array below",
		},
		"tasks": map[string]any{
			"type":        "array",
			"description": "Tasks to append (or the full list when replace is true)",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id":          map[string]any{"type": "string", "description": "Stable id (optional; generated if omitted)"},
					"subject":     map[string]any{"type": "string", "description": "Short title"},
					"description": map[string]any{"type": "string", "description": "Longer context"},
					"status":      map[string]any{"type": "string", "description": "pending | in_progress | completed | cancelled"},
					"owner":       map[string]any{"type": "string"},
					"depends_on":  map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
					"metadata":    map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
				},
				"required": []string{"subject"},
			},
		},
	}, []string{"tasks"})
}

func (TaskCreateTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in taskCreateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	if len(in.Tasks) == 0 {
		return "", fmt.Errorf("tasks array must not be empty")
	}
	ins := make([]tasks.CreateInput, 0, len(in.Tasks))
	for _, row := range in.Tasks {
		ins = append(ins, tasks.CreateInput{
			ID:          row.ID,
			Subject:     row.Subject,
			Description: row.Description,
			Status:      row.Status,
			Owner:       row.Owner,
			DependsOn:   row.DependsOn,
			Metadata:    row.Metadata,
		})
	}
	return tasks.CreateWithInstruction(tctx.CWD, tctx.InstructionRoot, tctx.WorkspaceFlat, in.Replace, ins)
}
