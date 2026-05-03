package builtin

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const NameTodo = "todo"

const todoFileName = "todo.json"

// MaxTodoFileBytes caps todo.json size on disk.
const MaxTodoFileBytes = 256 * 1024

type todoItem struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type todoFile struct {
	Version int        `json:"version"`
	Items   []todoItem `json:"items"`
}

type todoToolIn struct {
	Action string `json:"action" jsonschema:"description=list | add | toggle | remove | clear,required=true"`
	Title  string `json:"title,omitempty" jsonschema:"description=add: task title"`
	ID     string `json:"id,omitempty" jsonschema:"description=toggle/remove: item id"`
	Done   *bool  `json:"done,omitempty" jsonschema:"description=toggle: optional explicit done state"`
}

// InferTodo maintains InstructionRoot/todo.json (session-local task list).
func InferTodo(instructionRoot string) (tool.InvokableTool, error) {
	root := strings.TrimSpace(instructionRoot)
	if root == "" {
		return nil, fmt.Errorf("%s: instruction root required", NameTodo)
	}
	path := filepath.Join(root, todoFileName)
	return utils.InferTool(NameTodo,
		"Maintain session todo.json at the instruction root: list | add | toggle | remove | clear.",
		func(ctx context.Context, in todoToolIn) (string, error) {
			switch strings.ToLower(strings.TrimSpace(in.Action)) {
			case "list":
				return todoList(path)
			case "add":
				return todoAdd(path, in.Title)
			case "toggle":
				return todoToggle(path, in.ID, in.Done)
			case "remove":
				return todoRemove(path, in.ID)
			case "clear":
				return todoClear(path)
			default:
				return "", fmt.Errorf("todo: unknown action %q", in.Action)
			}
		})
}

func todoLoad(path string) (todoFile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return todoFile{Version: 1, Items: nil}, nil
		}
		return todoFile{}, err
	}
	if len(b) > MaxTodoFileBytes {
		return todoFile{}, fmt.Errorf("todo: file too large")
	}
	var f todoFile
	if err := json.Unmarshal(b, &f); err != nil {
		return todoFile{}, err
	}
	if f.Version == 0 {
		f.Version = 1
	}
	return f, nil
}

func todoSave(path string, f todoFile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if len(data) > MaxTodoFileBytes {
		return fmt.Errorf("todo: serialized size exceeds limit")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func todoList(path string) (string, error) {
	f, err := todoLoad(path)
	if err != nil {
		return "", err
	}
	if len(f.Items) == 0 {
		return "(no tasks)", nil
	}
	var b strings.Builder
	for _, it := range f.Items {
		mark := " "
		if it.Done {
			mark = "x"
		}
		fmt.Fprintf(&b, "- [%s] %s — `%s`\n", mark, it.Title, it.ID)
	}
	return strings.TrimSpace(b.String()), nil
}

func todoAdd(path, title string) (string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", fmt.Errorf("todo: add requires title")
	}
	f, err := todoLoad(path)
	if err != nil {
		return "", err
	}
	id := newTodoID()
	f.Items = append(f.Items, todoItem{ID: id, Title: title, Done: false})
	if err := todoSave(path, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("added task %q id=%s", title, id), nil
}

func todoToggle(path, id string, done *bool) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("todo: toggle requires id")
	}
	f, err := todoLoad(path)
	if err != nil {
		return "", err
	}
	for i := range f.Items {
		if f.Items[i].ID == id {
			if done != nil {
				f.Items[i].Done = *done
			} else {
				f.Items[i].Done = !f.Items[i].Done
			}
			if err := todoSave(path, f); err != nil {
				return "", err
			}
			return fmt.Sprintf("task %s done=%v", id, f.Items[i].Done), nil
		}
	}
	return "", fmt.Errorf("todo: unknown id %q", id)
}

func todoRemove(path, id string) (string, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return "", fmt.Errorf("todo: remove requires id")
	}
	f, err := todoLoad(path)
	if err != nil {
		return "", err
	}
	out := f.Items[:0]
	found := false
	for _, it := range f.Items {
		if it.ID == id {
			found = true
			continue
		}
		out = append(out, it)
	}
	if !found {
		return "", fmt.Errorf("todo: unknown id %q", id)
	}
	f.Items = out
	if err := todoSave(path, f); err != nil {
		return "", err
	}
	return fmt.Sprintf("removed %s", id), nil
}

func todoClear(path string) (string, error) {
	f := todoFile{Version: 1, Items: nil}
	if err := todoSave(path, f); err != nil {
		return "", err
	}
	return "cleared all tasks", nil
}

func newTodoID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("t-%d", time.Now().UnixNano())
	}
	return "t-" + hex.EncodeToString(b[:])
}
