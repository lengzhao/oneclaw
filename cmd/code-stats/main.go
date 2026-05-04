package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var skipDirs = map[string]struct{}{
	"vendor":       {},
	".git":         {},
	"node_modules": {},
	".cache":       {},
}

type bucket struct {
	Files      int
	LinesTotal int
	LinesBlank int
}

func (b bucket) nonempty() int { return b.LinesTotal - b.LinesBlank }

type reportBucket struct {
	Files         int `json:"files"`
	LinesTotal    int `json:"lines_total"`
	LinesBlank    int `json:"lines_blank"`
	LinesNonempty int `json:"lines_nonempty"`
}

type report struct {
	Root       string       `json:"root"`
	Production reportBucket `json:"production"`
	Test       reportBucket `json:"test"`
	All        reportBucket `json:"all"`
}

func isTestGo(path string) bool {
	return strings.HasSuffix(filepath.Base(path), "_test.go")
}

func collectGoFiles(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if _, skip := skipDirs[name]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// splitLines mirrors Python str.splitlines() for \n-normalized text:
// normalize CRLF/CR to \n, split on \n, drop a single trailing empty segment.
func splitLines(data []byte) [][]byte {
	s := bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	s = bytes.ReplaceAll(s, []byte("\r"), []byte("\n"))
	parts := bytes.Split(s, []byte("\n"))
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func countFile(path string) (total, blank int) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "skip %s: %v\n", path, err)
		return 0, 0
	}
	for _, line := range splitLines(data) {
		total++
		if len(bytes.TrimSpace(line)) == 0 {
			blank++
		}
	}
	return total, blank
}

func bucketToReport(b bucket) reportBucket {
	return reportBucket{
		Files:         b.Files,
		LinesTotal:    b.LinesTotal,
		LinesBlank:    b.LinesBlank,
		LinesNonempty: b.nonempty(),
	}
}

func main() {
	jsonOut := flag.Bool("json", false, "print summary as JSON")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: %s [--json] [根目录]\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}
	flag.Parse()

	root := "."
	if flag.NArg() > 0 {
		root = flag.Arg(0)
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	paths, err := collectGoFiles(abs)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var prod, test bucket
	for _, path := range paths {
		tot, bl := countFile(path)
		if tot == 0 && bl == 0 {
			continue
		}
		b := &prod
		if isTestGo(path) {
			b = &test
		}
		b.Files++
		b.LinesTotal += tot
		b.LinesBlank += bl
	}

	all := bucket{
		Files:      prod.Files + test.Files,
		LinesTotal: prod.LinesTotal + test.LinesTotal,
		LinesBlank: prod.LinesBlank + test.LinesBlank,
	}

	rep := report{
		Root:       abs,
		Production: bucketToReport(prod),
		Test:       bucketToReport(test),
		All:        bucketToReport(all),
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	fmt.Printf("root: %s\n\n", abs)
	printRow("production", prod)
	printRow("test", test)
	printRow("all", all)
}

func printRow(label string, b bucket) {
	fmt.Printf("%-12s  files=%5d  total=%7d  blank=%7d  nonempty=%7d\n",
		label, b.Files, b.LinesTotal, b.LinesBlank, b.nonempty())
}
