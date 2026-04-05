package loop

import (
	"strings"
	"testing"

	"github.com/lengzhao/oneclaw/prompts"
)

func TestCompactEnvelopeTemplateLayout(t *testing.T) {
	const kind = "compact_boundary"
	const ts = "2006-01-02T15:04:05Z"
	summary := "  line one  "
	want := "[oneclaw:" + kind + " ts=" + ts + "]\n" +
		"Earlier conversation (omitted from context for byte budget). Heuristic recap — verify with tools if needed:\n\n" +
		strings.TrimSpace(summary) + "\n\n[/oneclaw:" + kind + "]\n"
	got, err := prompts.Render(prompts.NameCompactEnvelope, struct {
		Kind      string
		Timestamp string
		Summary   string
	}{
		Kind:      kind,
		Timestamp: ts,
		Summary:   strings.TrimSpace(summary),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("compact_envelope mismatch\nwant %q\ngot  %q", want, got)
	}
}
