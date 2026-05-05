package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudwego/eino/components/tool"
)

func TestListDirTool_flat(t *testing.T) {
	ctx := context.Background()
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "d"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolListDir}); err != nil {
		t.Fatal(err)
	}
	ts, err := r.FilterByNames([]string{ToolListDir})
	if err != nil {
		t.Fatal(err)
	}
	inv := ts[0].(tool.InvokableTool)
	out, err := inv.InvokableRun(ctx, `{}`)
	if err != nil {
		t.Fatal(err)
	}
	if out != "a.txt\nd/" {
		t.Fatalf("got %q", out)
	}
}

func TestGlobTool_recursive(t *testing.T) {
	ctx := context.Background()
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "a", "b", "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "root.go"), []byte("p"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolGlob}); err != nil {
		t.Fatal(err)
	}
	ts, err := r.FilterByNames([]string{ToolGlob})
	if err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]any{"pattern": "*.txt", "recursive": true})
	out, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(args))
	if err != nil {
		t.Fatal(err)
	}
	if out != "a/b/x.txt" {
		t.Fatalf("got %q", out)
	}
}

func TestWriteFile_replaceExactlyOnce(t *testing.T) {
	ctx := context.Background()
	ws := t.TempDir()
	path := filepath.Join(ws, "t.go")
	if err := os.WriteFile(path, []byte("alpha beta gamma"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolWriteFile, ToolReadFile}); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{
		"path":      "t.go",
		"operation": "replace",
		"old_text":  "beta",
		"content":   "BETA",
	})
	ts, _ := r.FilterByNames([]string{ToolWriteFile})
	if _, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(args)); err != nil {
		t.Fatal(err)
	}
	readArgs, _ := json.Marshal(map[string]string{"path": "t.go"})
	ts, _ = r.FilterByNames([]string{ToolReadFile})
	body, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(readArgs))
	if err != nil || body != "alpha BETA gamma" {
		t.Fatalf("got %q err=%v", body, err)
	}
}

func TestWriteFile_replaceAmbiguous(t *testing.T) {
	ctx := context.Background()
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "x.txt"), []byte("aa aa"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolWriteFile}); err != nil {
		t.Fatal(err)
	}
	args, _ := json.Marshal(map[string]string{"path": "x.txt", "operation": "replace", "old_text": "aa", "content": "b"})
	ts, _ := r.FilterByNames([]string{ToolWriteFile})
	if _, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(args)); err == nil {
		t.Fatal("expected error for ambiguous old_text")
	}
}

func TestWriteReadAppend_roundTrip(t *testing.T) {
	ctx := context.Background()
	ws := t.TempDir()
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolReadFile, ToolWriteFile}); err != nil {
		t.Fatal(err)
	}

	writeArgs, _ := json.Marshal(map[string]string{"path": "nest/x.txt", "content": "hello"})
	ts, _ := r.FilterByNames([]string{ToolWriteFile})
	out, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(writeArgs))
	if err != nil {
		t.Fatal(err)
	}
	if out == "" {
		t.Fatal("empty write result")
	}

	readArgs, _ := json.Marshal(map[string]string{"path": "nest/x.txt"})
	ts, _ = r.FilterByNames([]string{ToolReadFile})
	body, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(readArgs))
	if err != nil || body != "hello" {
		t.Fatalf("read got %q err=%v", body, err)
	}

	appArgs, _ := json.Marshal(map[string]string{"path": "nest/x.txt", "operation": "append", "content": "\nworld"})
	ts, _ = r.FilterByNames([]string{ToolWriteFile})
	if _, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(appArgs)); err != nil {
		t.Fatal(err)
	}
	ts, _ = r.FilterByNames([]string{ToolReadFile})
	body, err = ts[0].(tool.InvokableTool).InvokableRun(ctx, string(readArgs))
	if err != nil || body != "hello\nworld" {
		t.Fatalf("after append got %q err=%v", body, err)
	}
}

func TestReadWriteFile_allowsAbsolutePathOutsideWorkspace(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	outside := filepath.Join(tmp, "outside.txt")
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolReadFile, ToolWriteFile}); err != nil {
		t.Fatal(err)
	}

	writeArgs, _ := json.Marshal(map[string]string{"path": outside, "content": "outside"})
	ts, _ := r.FilterByNames([]string{ToolWriteFile})
	if _, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(writeArgs)); err != nil {
		t.Fatal(err)
	}

	readArgs, _ := json.Marshal(map[string]string{"path": outside})
	ts, _ = r.FilterByNames([]string{ToolReadFile})
	body, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(readArgs))
	if err != nil || body != "outside" {
		t.Fatalf("absolute read got %q err=%v", body, err)
	}
}

func TestReadWriteFile_allowsParentTraversalOutsideWorkspace(t *testing.T) {
	ctx := context.Background()
	tmp := t.TempDir()
	ws := filepath.Join(tmp, "workspace")
	r := NewRegistry(ws)
	if err := RegisterBuiltinsNamed(r, []string{ToolReadFile, ToolWriteFile}); err != nil {
		t.Fatal(err)
	}

	writeArgs, _ := json.Marshal(map[string]string{"path": "../parent.txt", "content": "parent"})
	ts, _ := r.FilterByNames([]string{ToolWriteFile})
	if _, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(writeArgs)); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "parent.txt")); err != nil {
		t.Fatal(err)
	}

	readArgs, _ := json.Marshal(map[string]string{"path": "../parent.txt"})
	ts, _ = r.FilterByNames([]string{ToolReadFile})
	body, err := ts[0].(tool.InvokableTool).InvokableRun(ctx, string(readArgs))
	if err != nil || body != "parent" {
		t.Fatalf("parent traversal read got %q err=%v", body, err)
	}
}
