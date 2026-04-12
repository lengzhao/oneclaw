package session

import "testing"

func TestIsStopSlashCommand(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"/stop", true},
		{"  /stop  ", true},
		{"/stop extra", false},
		{"/stop/", false},
		{"/help", false},
		{"stop", false},
	}
	for _, tc := range tests {
		if got := IsStopSlashCommand(tc.in); got != tc.want {
			t.Errorf("IsStopSlashCommand(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
