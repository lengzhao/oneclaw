package memory

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ExportSessionSnapshot copies selected `.oneclaw` artifacts into outDir for offline review or bug reports.
// Large `artifacts/` under `.oneclaw` is skipped by default to avoid huge DOM dumps.
func ExportSessionSnapshot(cwd, outDir string) error {
	if cwd == "" {
		return fmt.Errorf("memory: empty cwd")
	}
	outDir = filepath.Clean(outDir)
	if outDir == "" || outDir == "." {
		return fmt.Errorf("memory: invalid export output directory")
	}
	srcRoot := filepath.Join(cwd, DotDir)
	if st, err := os.Stat(srcRoot); err != nil {
		return fmt.Errorf("stat %s: %w", srcRoot, err)
	} else if !st.IsDir() {
		return fmt.Errorf("not a directory: %s", srcRoot)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return fmt.Errorf("mkdir export dir: %w", err)
	}
	readme := filepath.Join(outDir, "README_EXPORT.txt")
	summary := fmt.Sprintf(
		"oneclaw session export\n"+
			"generated_utc: %s\n"+
			"source_cwd: %s\n"+
			"source_dotdir: %s\n\n"+
			"Includes: transcript, working_transcript, tasks, config, memory/, sidechain/ (if present).\n"+
			"Excludes: .oneclaw/artifacts/ (use your own copy if needed).\n",
		time.Now().UTC().Format(time.RFC3339), cwd, srcRoot,
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
	for _, dir := range []string{"memory", "sidechain"} {
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
