// go-loc-stats: count Go source lines by category (blank / comment-only / code) and by prod vs _test.go.
package main

import (
	"flag"
	"fmt"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

type lineBits struct {
	hasCode    bool
	hasComment bool
}

type tally struct {
	blank       int
	commentOnly int
	code        int
	physical    int
}

type stats struct {
	prod tally
	test tally
}

func main() {
	root := flag.String("root", ".", "root directory to scan")
	skipDirs := flag.String("skip", "vendor,.git,testdata,node_modules", "comma-separated dir names to skip when walking")
	flag.Parse()

	skip := splitSkip(*skipDirs)
	dir := filepath.Clean(*root)

	var st stats
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if path != dir && skip[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		analyzeFile(path, strings.HasSuffix(filepath.Base(path), "_test.go"), &st)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk: %v\n", err)
		os.Exit(1)
	}

	printReport(&st)
}

func splitSkip(s string) map[string]bool {
	m := map[string]bool{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = true
		}
	}
	return m
}

func analyzeFile(path string, isTest bool, st *stats) {
	src, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", path, err)
		return
	}

	text := strings.ReplaceAll(string(src), "\r\n", "\n")
	rawLines := strings.Split(text, "\n")
	n := len(rawLines)
	if n == 0 {
		return
	}

	lines := make([]lineBits, n+1)

	fset := token.NewFileSet()
	file := fset.AddFile(path, -1, len(src))

	var s scanner.Scanner
	s.Init(file, src, nil, scanner.ScanComments)

	for {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}
		startOff := file.Offset(pos)
		endOff := tokenEndOffset(tok, lit, startOff)

		startLine := fset.Position(pos).Line
		endLine := fset.Position(file.Pos(endOff)).Line
		if endLine < startLine {
			endLine = startLine
		}

		isComment := tok == token.COMMENT
		isCode := !isComment && tok != token.SEMICOLON
		if tok == token.ILLEGAL {
			isCode = true
			isComment = false
		}

		for ln := startLine; ln <= endLine && ln <= n; ln++ {
			if isComment {
				lines[ln].hasComment = true
			}
			if isCode {
				lines[ln].hasCode = true
			}
		}
	}

	t := &st.prod
	if isTest {
		t = &st.test
	}

	for i, ln := range rawLines {
		lineNo := i + 1
		t.physical++
		if strings.TrimSpace(ln) == "" {
			t.blank++
			continue
		}
		b := lines[lineNo]
		if b.hasCode {
			t.code++
			continue
		}
		if b.hasComment {
			t.commentOnly++
			continue
		}
		t.code++
	}
}

func tokenEndOffset(tok token.Token, lit string, startOff int) int {
	switch tok {
	case token.COMMENT, token.STRING, token.IDENT, token.INT, token.FLOAT, token.IMAG, token.CHAR:
		if len(lit) > 0 {
			return startOff + len(lit) - 1
		}
	case token.SEMICOLON:
		return startOff
	}
	if lit != "" {
		return startOff + len(lit) - 1
	}
	s := tok.String()
	if s != "" && s != "ILLEGAL" {
		return startOff + len(s) - 1
	}
	return startOff
}

func printReport(st *stats) {
	fmt.Println("Go 代码量统计（基于 go/scanner；//go:build 等指令行属于 COMMENT，计入「仅注释」）")
	fmt.Println()

	header := []string{"类别", "正式代码 (*.go)", "测试 (*_test.go)", "合计"}
	colw := []int{30, 18, 22, 12}
	printRow := func(cells ...string) {
		for i, c := range cells {
			pad := colw[i] - utf8Len(c)
			if pad < 0 {
				pad = 0
			}
			fmt.Print(c + strings.Repeat(" ", pad))
		}
		fmt.Println()
	}

	printRow(header...)
	fmt.Println(strings.Repeat("-", 84))

	sumBlank := st.prod.blank + st.test.blank
	sumComment := st.prod.commentOnly + st.test.commentOnly
	sumCode := st.prod.code + st.test.code
	sumPhys := st.prod.physical + st.test.physical

	printRow("物理行（总行）",
		fmt.Sprintf("%d", st.prod.physical),
		fmt.Sprintf("%d", st.test.physical),
		fmt.Sprintf("%d", sumPhys))
	printRow("空行",
		fmt.Sprintf("%d", st.prod.blank),
		fmt.Sprintf("%d", st.test.blank),
		fmt.Sprintf("%d", sumBlank))
	printRow("仅注释行",
		fmt.Sprintf("%d", st.prod.commentOnly),
		fmt.Sprintf("%d", st.test.commentOnly),
		fmt.Sprintf("%d", sumComment))
	printRow("代码行（至少一个非注释 token）",
		fmt.Sprintf("%d", st.prod.code),
		fmt.Sprintf("%d", st.test.code),
		fmt.Sprintf("%d", sumCode))
	fmt.Println()
	fmt.Println("说明：")
	fmt.Println("  - 同一行既有代码又有 // 注释时，计为「代码行」。")
	fmt.Println("  - 默认跳过目录见 -skip（vendor,.git,testdata,node_modules）。")
}

func utf8Len(s string) int {
	return len([]rune(s))
}
