package memory

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTokenizeRecall_hanBigrams(t *testing.T) {
	terms := tokenizeRecall("用户登录配置")
	want := []string{"用户", "户登", "登录", "录配", "配置"}
	if len(terms) != len(want) {
		t.Fatalf("got %d terms %v, want %d %v", len(terms), terms, len(want), want)
	}
	for i, w := range want {
		if terms[i] != w {
			t.Fatalf("terms[%d]=%q, want %q (full %v)", i, terms[i], w, terms)
		}
	}
}

func TestTokenizeRecall_mixedLatinHan(t *testing.T) {
	terms := tokenizeRecall("查一下 login 流程问题")
	seen := make(map[string]struct{})
	for _, x := range terms {
		seen[x] = struct{}{}
	}
	if _, ok := seen["login"]; !ok {
		t.Fatalf("missing latin token login: %v", terms)
	}
	if _, ok := seen["流程"]; !ok {
		t.Fatalf("missing bigram 流程: %v", terms)
	}
	if _, ok := seen["问题"]; !ok {
		t.Fatalf("missing bigram 问题: %v", terms)
	}
}

func TestTokenizeRecall_singleHanDropped(t *testing.T) {
	terms := tokenizeRecall("查")
	if len(terms) != 0 {
		t.Fatalf("expected no terms for single Han, got %v", terms)
	}
}

func TestTokenizeRecall_englishCompatible(t *testing.T) {
	terms := tokenizeRecall("What about zebrarecall_e2e_30?")
	seen := make(map[string]struct{})
	for _, x := range terms {
		seen[x] = struct{}{}
	}
	if _, ok := seen["zebrarecall"]; !ok {
		t.Fatalf("expected zebrarecall token, got %v", terms)
	}
	if _, ok := seen["e2e"]; !ok {
		t.Fatalf("expected e2e token, got %v", terms)
	}
}

func TestTokenizeRecall_dedupe(t *testing.T) {
	terms := tokenizeRecall("用户用户")
	seen := make(map[string]struct{})
	for _, x := range terms {
		if _, ok := seen[x]; ok {
			t.Fatalf("duplicate term %q in %v", x, terms)
		}
		seen[x] = struct{}{}
	}
	// 用户用户 → 用户 户用 用户 户用，去重后仅 用户、户用
	if len(terms) != 2 {
		t.Fatalf("want 2 unique bigrams, got %d: %v", len(terms), terms)
	}
}

func TestTokenizeRecall_termCap(t *testing.T) {
	var b strings.Builder
	for i := range maxRecallTermCount + 8 {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString("tok")
		b.WriteByte(byte('a' + (i % 26)))
		b.WriteByte(byte('a' + ((i / 26) % 26)))
	}
	terms := tokenizeRecall(b.String())
	if len(terms) != maxRecallTermCount {
		t.Fatalf("want %d terms capped, got %d", maxRecallTermCount, len(terms))
	}
}

func TestSelectRecall_skipsRootMemoryMdOnly(t *testing.T) {
	cwd := t.TempDir()
	home := t.TempDir()
	lay := DefaultLayout(cwd, home)
	proj := lay.Project
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "MEMORY.md"), []byte("standing rules secretphrase\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "2026-04-07.md"), []byte("episodic recallmarker_fact here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body, _ := SelectRecall(lay, "recallmarker_fact", nil, 12_000)
	if !strings.Contains(body, "recallmarker_fact") {
		t.Fatalf("expected episodic file in recall, got:\n%s", body)
	}
	if strings.Contains(body, "secretphrase") {
		t.Fatalf("root MEMORY.md should not be recalled, got:\n%s", body)
	}
}
