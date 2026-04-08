package memory

import (
	"strings"
	"testing"
)

func TestBodyStartByteOffset_matchesStripYAMLFrontmatter(t *testing.T) {
	cases := []struct {
		name string
		raw  string
	}{
		{"plain", "hello\n"},
		{"bom", "\ufeffhello\n"},
		{"frontmatter", "---\ntitle: x\n---\n\nbody here\n"},
		{"frontmatter_crlf_close", "---\ntitle: x\n---\r\nbody\r\n"},
		{"no_closing_fence", "---\nnotclosed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			start := BodyStartByteOffset(tc.raw)
			got := tc.raw[start:]
			want := StripYAMLFrontmatter(tc.raw)
			if got != want {
				t.Fatalf("raw[%d:] != StripYAMLFrontmatter: got %q want %q", start, got, want)
			}
		})
	}
}

func TestBodyStartByteOffset_frontmatterNeedleOffset(t *testing.T) {
	raw := "---\nt: 1\n---\n\nneedle_here_suffix\n"
	start := BodyStartByteOffset(raw)
	needle := "needle_here"
	idxFile := strings.Index(raw, needle)
	if idxFile < 0 {
		t.Fatal("fixture broken")
	}
	body := raw[start:]
	idxBody := strings.Index(body, needle)
	if idxBody < 0 {
		t.Fatal("needle missing from body slice")
	}
	if start+idxBody != idxFile {
		t.Fatalf("file index %d != bodyBase+local %d+%d", idxFile, start, idxBody)
	}
}
