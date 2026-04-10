package logx

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInit_writesToFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "out.log")
	close := Init("info", "text", path)
	defer close()

	slog.Info("hello_file_test")

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "hello_file_test") {
		t.Fatalf("log file missing message: %q", string(b))
	}
}
