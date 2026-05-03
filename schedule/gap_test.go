package schedule

import "testing"

func TestApplyMinFireGap(t *testing.T) {
	if g := applyMinFireGap(50, 100); g != 160 {
		t.Fatalf("next below floor: got %d want 160", g)
	}
	if g := applyMinFireGap(200, 100); g != 200 {
		t.Fatalf("next above floor: got %d want 200", g)
	}
}
