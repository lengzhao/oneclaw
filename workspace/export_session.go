package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportSessionSnapshot copies host data from dataRoot (typically ~/.oneclaw) into outDir for offline review or bug reports.
// Output keeps a `.oneclaw/` subdirectory mirroring the old export layout for compatibility.
// Large `artifacts/` trees are skipped by default to avoid huge DOM dumps.
func ExportSessionSnapshot(dataRoot, outDir string) error {
	if dataRoot == "" {
		return fmt.Errorf("workspace: empty data root")
	}
	srcRoot := filepath.Clean(dataRoot)
	if st, err := os.Stat(srcRoot); err != nil {
		return fmt.Errorf("stat %s: %w", srcRoot, err)
	} else if !st.IsDir() {
		return fmt.Errorf("not a directory: %s", srcRoot)
	}
	outDir = filepath.Clean(outDir)
	if outDir == "" || outDir == "." {
		return fmt.Errorf("workspace: invalid export output directory")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir export dir: %w", err)
	}
	readme := filepath.Join(outDir, "README_EXPORT.txt")
	summary := fmt.Sprintf(
		"oneclaw session export\n"+
			"generated_utc: %s\n"+
			"source_data_root: %s\n\n"+
			"Includes: top-level config/jobs, memory/, sessions/, sidechain/ when present.\n"+
			"Excludes: artifacts/ (use your own copy if needed).\n",
		time.Now().UTC().Format(time.RFC3339), srcRoot,
	)
	if err := os.WriteFile(readme, []byte(summary), 0o644); err != nil {
		return fmt.Errorf("write readme: %w", err)
	}
	dstDot := filepath.Join(outDir, DotDir)
	files := []string{
		"transcript.json",
		"working_transcript.json",
		"tasks.json",
		"config.yaml",
		"scheduled_maintain_state.json",
		"scheduled_jobs.json",
	}
	for _, name := range files {
		if err := copyFileIfExists(filepath.Join(srcRoot, name), filepath.Join(dstDot, name)); err != nil {
			return err
		}
	}
	for _, dir := range []string{"memory", "sessions", "sidechain", "media"} {
		sp := filepath.Join(srcRoot, dir)
		if _, err := os.Stat(sp); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if err := copyDirSkipArtifacts(sp, filepath.Join(dstDot, dir)); err != nil {
			return fmt.Errorf("copy %s: %w", dir, err)
		}
	}
	return nil
}

func copyFileIfExists(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read %s: %w", src, err)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

func copyDirSkipArtifacts(srcRoot, dstRoot string) error {
	return filepath.Walk(srcRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		parts := strings.Split(rel, string(os.PathSeparator))
		for _, p := range parts {
			if p == "artifacts" {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		dst := filepath.Join(dstRoot, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		return copyFileFull(path, dst)
	})
}

func copyFileFull(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
