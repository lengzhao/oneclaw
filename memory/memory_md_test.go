package memory

import (
	"bytes"
	"strings"
	"testing"
)

func TestTruncateMEMORYMDForInjection_short(t *testing.T) {
	in := []byte("hello")
	out := TruncateMEMORYMDForInjection(in)
	if !bytes.Equal(out, in) {
		t.Fatalf("got %q want %q", out, in)
	}
}

func TestTruncateMEMORYMDForInjection_long(t *testing.T) {
	s := strings.Repeat("a", MEMORYMDMaxBytes+100)
	out := TruncateMEMORYMDForInjection([]byte(s))
	if len(out) > MEMORYMDMaxBytes {
		t.Fatalf("len %d > max %d", len(out), MEMORYMDMaxBytes)
	}
	if !strings.Contains(string(out), "truncated") {
		t.Fatalf("missing truncation marker")
	}
}
