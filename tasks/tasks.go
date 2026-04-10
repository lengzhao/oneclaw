// Package tasks persists session-scoped work items for long runs and resume (Claude Code–style task list).
package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lengzhao/oneclaw/budget"
	"github.com/lengzhao/oneclaw/memory"
	"github.com/lengzhao/oneclaw/rtopts"
)

const fileName = "tasks.json"

// Path returns the absolute path to <cwd>/.oneclaw/tasks.json.
func Path(cwd string) string {
	return filepath.Join(cwd, memory.DotDir, fileName)
}

// Disabled reports features.disable_tasks from config.
func Disabled() bool {
	return rtopts.Current().DisableTasks
}

// File is the on-disk JSON shape.
type File struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
	Items     []Item    `json:"items"`
}

// Item is one row in the task list.
type Item struct {
	ID          string            `json:"id"`
	Subject     string            `json:"subject"`
	Description string            `json:"description,omitempty"`
	Status      string            `json:"status"`
	Owner       string            `json:"owner,omitempty"`
	DependsOn   []string          `json:"depends_on,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

var fileMu sync.Mutex

func newID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "ts_" + hex.EncodeToString(b[:])
}

func normalizeStatus(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "":
		return "pending", nil
	case "pending", "in_progress", "completed", "cancelled":
		return s, nil
	default:
		return "", fmt.Errorf("invalid status %q (want pending, in_progress, completed, cancelled)", s)
	}
}

// Read loads tasks from path. Missing file yields an empty File (no error).
func Read(path string) (*File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &File{Version: 1, Items: nil}, nil
		}
		return nil, err
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return &f, nil
}

func write(path string, f *File) error {
	f.Version = 1
	f.UpdatedAt = time.Now().UTC()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(dir, ".tasks-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

// CreateInput is one task row for task_create.
type CreateInput struct {
	ID          string
	Subject     string
	Description string
	Status      string
	Owner       string
	DependsOn   []string
	Metadata    map[string]string
}

// Create appends tasks or replaces the list when replace is true.
func Create(cwd string, replace bool, inputs []CreateInput) (string, error) {
	if Disabled() {
		return "", fmt.Errorf("tasks are disabled (features.disable_tasks in config)")
	}
	path := Path(cwd)
	fileMu.Lock()
	defer fileMu.Unlock()
	f, err := Read(path)
	if err != nil {
		return "", err
	}
	seen := map[string]struct{}{}
	var out []Item
	if !replace {
		for _, it := range f.Items {
			seen[it.ID] = struct{}{}
		}
		out = append(out, f.Items...)
	}
	for _, in := range inputs {
		sub := strings.TrimSpace(in.Subject)
		if sub == "" {
			return "", fmt.Errorf("task subject is required")
		}
		id := strings.TrimSpace(in.ID)
		if id == "" {
			for {
				id = newID()
				if _, ok := seen[id]; !ok {
					break
				}
			}
		}
		if _, ok := seen[id]; ok {
			return "", fmt.Errorf("duplicate task id %q", id)
		}
		seen[id] = struct{}{}
		st, err := normalizeStatus(in.Status)
		if err != nil {
			return "", err
		}
		out = append(out, Item{
			ID:          id,
			Subject:     sub,
			Description: strings.TrimSpace(in.Description),
			Status:      st,
			Owner:       strings.TrimSpace(in.Owner),
			DependsOn:   append([]string(nil), in.DependsOn...),
			Metadata:    copyMeta(in.Metadata),
		})
	}
	f.Items = out
	if err := write(path, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("saved %d task(s) to %s (total %d)", len(inputs), path, len(f.Items)), nil
}

func copyMeta(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		out[k] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// UpdatePatch updates one task by id. Only non-nil / non-empty fields apply.
type UpdatePatch struct {
	Status      *string
	Subject     *string
	Description *string
	Owner       *string
	DependsOn   *[]string
	Metadata    map[string]string
	// CompletionEvidence is optional text required when transitioning to completed (see Update).
	// Usually set by the task_update tool from JSON field completion_evidence.
	CompletionEvidence *string
}

// Update mutates a single task.
func Update(cwd, taskID string, patch UpdatePatch) (string, error) {
	if Disabled() {
		return "", fmt.Errorf("tasks are disabled (features.disable_tasks in config)")
	}
	id := strings.TrimSpace(taskID)
	if id == "" {
		return "", fmt.Errorf("task_id is required")
	}
	path := Path(cwd)
	fileMu.Lock()
	defer fileMu.Unlock()
	f, err := Read(path)
	if err != nil {
		return "", err
	}
	idx := -1
	for i := range f.Items {
		if f.Items[i].ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return "", fmt.Errorf("unknown task_id %q", id)
	}
	if patch.Status == nil && patch.Subject == nil && patch.Description == nil && patch.Owner == nil && patch.DependsOn == nil && patch.Metadata == nil && patch.CompletionEvidence == nil {
		return "no fields to update", nil
	}
	it := f.Items[idx]
	oldStatus := it.Status
	var newStatus string = oldStatus
	if patch.Status != nil {
		st, err := normalizeStatus(*patch.Status)
		if err != nil {
			return "", err
		}
		newStatus = st
	}
	if newStatus == "completed" && oldStatus != "completed" {
		ev := ""
		if patch.CompletionEvidence != nil {
			ev = strings.TrimSpace(*patch.CompletionEvidence)
		}
		if ev == "" && patch.Metadata != nil {
			ev = strings.TrimSpace(patch.Metadata["completion_evidence"])
		}
		if ev == "" {
			return "", fmt.Errorf("task %q: status cannot move to completed without completion_evidence (tool field completion_evidence or metadata.completion_evidence): one short sentence of verified outcome", id)
		}
	}
	if patch.Status != nil {
		it.Status = newStatus
	}
	if patch.Subject != nil {
		s := strings.TrimSpace(*patch.Subject)
		if s == "" {
			return "", fmt.Errorf("subject cannot be empty")
		}
		it.Subject = s
	}
	if patch.Description != nil {
		it.Description = strings.TrimSpace(*patch.Description)
	}
	if patch.Owner != nil {
		it.Owner = strings.TrimSpace(*patch.Owner)
	}
	if patch.DependsOn != nil {
		it.DependsOn = append([]string(nil), (*patch.DependsOn)...)
	}
	if patch.Metadata != nil {
		if it.Metadata == nil {
			it.Metadata = make(map[string]string)
		}
		for k, v := range patch.Metadata {
			k = strings.TrimSpace(k)
			if k == "" {
				continue
			}
			it.Metadata[k] = v
		}
		if len(it.Metadata) == 0 {
			it.Metadata = nil
		}
	}
	if patch.CompletionEvidence != nil {
		if t := strings.TrimSpace(*patch.CompletionEvidence); t != "" {
			if it.Metadata == nil {
				it.Metadata = make(map[string]string)
			}
			it.Metadata["completion_evidence"] = t
		}
	}
	f.Items[idx] = it
	if err := write(path, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("updated task %s (%s): %s", it.ID, it.Status, it.Subject), nil
}

const maxSystemItems = 48
const maxSystemBytes = 12000

func taskLineForPrompt(it *Item) string {
	line := fmt.Sprintf("- **[%s]** `%s` — %s", it.Status, it.ID, it.Subject)
	if it.Description != "" {
		line += " — " + it.Description
	}
	if len(it.DependsOn) > 0 {
		line += fmt.Sprintf(" (depends_on: %s)", strings.Join(it.DependsOn, ", "))
	}
	if it.Owner != "" {
		line += fmt.Sprintf(" [owner: %s]", it.Owner)
	}
	return line
}

// PromptTaskLines returns tasks.json path and one markdown bullet per task (same lines as SystemBlock).
// omitted is how many items exist beyond maxSystemItems. Empty result when disabled, missing file, or no items.
func PromptTaskLines(cwd string) (filePath string, lines []string, omitted int) {
	if Disabled() {
		return "", nil, 0
	}
	path := Path(cwd)
	f, err := Read(path)
	if err != nil || len(f.Items) == 0 {
		return "", nil, 0
	}
	n := len(f.Items)
	if n > maxSystemItems {
		omitted = n - maxSystemItems
		n = maxSystemItems
	}
	lines = make([]string, 0, n)
	for i := 0; i < n; i++ {
		it := f.Items[i]
		lines = append(lines, taskLineForPrompt(&it))
	}
	return path, lines, omitted
}

// SystemBlock returns a markdown section for the model (empty if disabled, missing file, or no items).
func SystemBlock(cwd string) string {
	path, lines, omitted := PromptTaskLines(cwd)
	if len(lines) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("\n## Task list (persisted)\n\n")
	b.WriteString("Structured work items are stored at `" + path + "`. Use `task_create` / `task_update` to keep them accurate across turns and restarts.\n\n")
	for _, ln := range lines {
		b.WriteString(ln)
		b.WriteByte('\n')
	}
	if omitted > 0 {
		fmt.Fprintf(&b, "\n… and %d more (not shown; see file or use tools)\n", omitted)
	}
	out := b.String()
	return budget.TruncateUTF8(out, maxSystemBytes)
}
