package paths

import "testing"

func TestSanitizeSessionPathSegment(t *testing.T) {
	if got := SanitizeSessionPathSegment(`../evil/name`); got != "evil_name" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeSessionPathSegment(""); got != "cli-default" {
		t.Fatalf("got %q", got)
	}
	if got := SanitizeSessionPathSegment("ok-1.x"); got != "ok-1.x" {
		t.Fatalf("got %q", got)
	}
}
