package structuredmem

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lzmem "github.com/lengzhao/memory"
	"gorm.io/gorm"
)

func sqlitePath(instructionRoot string) (string, error) {
	root := filepath.Clean(strings.TrimSpace(instructionRoot))
	if root == "" || root == "." {
		return "", fmt.Errorf("structuredmem: instruction root required")
	}
	dir := filepath.Join(root, "memory")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "structured_sqlite.db"), nil
}

// InitDB opens (and migrates) the per–instruction-root SQLite used by lengzhao/memory.
func InitDB(instructionRoot string) (*gorm.DB, error) {
	path, err := sqlitePath(instructionRoot)
	if err != nil {
		return nil, err
	}
	cfg := lzmem.DefaultConfig()
	cfg.Path = path
	cfg.AutoMigrate = true
	return lzmem.InitDB(cfg)
}

// OpenDB keeps backward compatibility for existing callers.
func OpenDB(instructionRoot string) (*gorm.DB, error) {
	return InitDB(instructionRoot)
}
