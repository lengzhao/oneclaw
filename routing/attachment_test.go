package routing

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeAttachments_Truncates(t *testing.T) {
	long := strings.Repeat("あ", maxAttachmentRunes+50)
	out := NormalizeAttachments([]Attachment{{Name: "x", Text: long}})
	if len(out) != 1 {
		t.Fatalf("len=%d", len(out))
	}
	if utf8.RuneCountInString(out[0].Text) <= maxAttachmentRunes {
		t.Fatalf("expected truncation, runes=%d", utf8.RuneCountInString(out[0].Text))
	}
	if !strings.Contains(out[0].Text, "[truncated:") {
		t.Fatal("missing notice")
	}
}

func TestNormalizeAttachments_EmptyName(t *testing.T) {
	out := NormalizeAttachments([]Attachment{{Text: "hi"}})
	if len(out) != 1 || out[0].Name != "attachment" {
		t.Fatalf("%+v", out)
	}
}
