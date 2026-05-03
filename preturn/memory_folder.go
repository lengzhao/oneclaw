package preturn

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// memoryFolderTreeDigest lists memory/ under instructionRoot (relative paths + byte sizes for .md files).
func memoryFolderTreeDigest(instructionRoot string, maxRunes int) string {
	root := filepath.Join(instructionRoot, "memory")
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return ""
	}
	var lines []string
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(instructionRoot, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() {
			if path == root {
				return nil
			}
			lines = append(lines, rel+"/")
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".md") {
			info, _ := d.Info()
			sz := int64(-1)
			if info != nil {
				sz = info.Size()
			}
			if sz >= 0 {
				lines = append(lines, rel+" ("+strconv.FormatInt(sz, 10)+" bytes)")
			} else {
				lines = append(lines, rel)
			}
		}
		return nil
	})
	if len(lines) == 0 {
		return ""
	}
	sort.Strings(lines)
	return truncateRunes(strings.Join(lines, "\n"), maxRunes)
}
