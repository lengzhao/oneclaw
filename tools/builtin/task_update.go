package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lengzhao/oneclaw/tasks"
	"github.com/lengzhao/oneclaw/toolctx"
	"github.com/openai/openai-go"
)

type taskUpdateInput struct {
	TaskID             string            `json:"task_id"`
	Status             string            `json:"status"`
	Subject            string            `json:"subject"`
	Description        string            `json:"description"`
	Owner              string            `json:"owner"`
	DependsOn          []string          `json:"depends_on"`
	Metadata           map[string]string `json:"metadata"`
	CompletionEvidence string            `json:"completion_evidence"`
}

// TaskUpdateTool updates one persisted task (status, text, owner, dependencies, metadata).
type TaskUpdateTool struct{}

func (TaskUpdateTool) Name() string          { return "task_update" }
func (TaskUpdateTool) ConcurrencySafe() bool { return false }

func (TaskUpdateTool) Description() string {
	return "Update a single task by id in <cwd>/.oneclaw/tasks.json. Mark in_progress when you start work and completed only when truly done. When setting status to completed, you must include completion_evidence (one short sentence: what was verified or delivered) or metadata.completion_evidence. Optional fields: subject, description, owner, depends_on, metadata (merged with existing metadata keys)."
}

func (TaskUpdateTool) Parameters() openai.FunctionParameters {
	return objectSchema(map[string]any{
		"task_id": map[string]any{
			"type":        "string",
			"description": "Task id from task_create or the task list",
		},
		"status": map[string]any{
			"type":        "string",
			"description": "pending | in_progress | completed | cancelled (omit to leave unchanged)",
		},
		"completion_evidence": map[string]any{
			"type":        "string",
			"description": "Required when moving to completed: one short sentence of verified outcome (tests passed, PR merged, user confirmed, etc.). Also accepted as metadata.completion_evidence.",
		},
		"subject": map[string]any{
			"type":        "string",
			"description": "New title (omit to leave unchanged)",
		},
		"description": map[string]any{
			"type":        "string",
			"description": "New body (omit to leave unchanged; empty string clears)",
		},
		"owner": map[string]any{
			"type":        "string",
			"description": "Optional owner label",
		},
		"depends_on": map[string]any{
			"type":        "array",
			"description": "Replace dependency ids (omit to leave unchanged)",
			"items":       map[string]any{"type": "string"},
		},
		"metadata": map[string]any{
			"type":                 "object",
			"description":          "String metadata merged into the task",
			"additionalProperties": map[string]any{"type": "string"},
		},
	}, []string{"task_id"})
}

func (TaskUpdateTool) Execute(ctx context.Context, input json.RawMessage, tctx *toolctx.Context) (string, error) {
	var in taskUpdateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(input, &raw); err != nil {
		return "", err
	}
	patch := tasks.UpdatePatch{}
	if st := strings.TrimSpace(in.Status); st != "" {
		patch.Status = &st
	}
	if _, ok := raw["subject"]; ok {
		s := strings.TrimSpace(in.Subject)
		if s == "" {
			return "", fmt.Errorf("subject cannot be empty when provided")
		}
		patch.Subject = &s
	}
	if _, ok := raw["description"]; ok {
		d := strings.TrimSpace(in.Description)
		patch.Description = &d
	}
	if _, ok := raw["owner"]; ok {
		o := strings.TrimSpace(in.Owner)
		patch.Owner = &o
	}
	if _, ok := raw["depends_on"]; ok {
		deps := append([]string(nil), in.DependsOn...)
		patch.DependsOn = &deps
	}
	if _, ok := raw["metadata"]; ok && len(in.Metadata) > 0 {
		patch.Metadata = in.Metadata
	}
	if _, ok := raw["completion_evidence"]; ok {
		s := strings.TrimSpace(in.CompletionEvidence)
		patch.CompletionEvidence = &s
	}
	return tasks.Update(tctx.CWD, in.TaskID, patch)
}
