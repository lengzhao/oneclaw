package routing

import "testing"

func TestParseLeadingSlash(t *testing.T) {
	tests := []struct {
		in      string
		wantCmd string
		wantArg string
		wantOK  bool
	}{
		{"", "", "", false},
		{"hello", "", "", false},
		{"/", "", "", false},
		{"/help", "help", "", true},
		{"  /Help  ", "help", "", true},
		{"/model extra", "model", "extra", true},
		{"//foo", "foo", "", true},
	}
	for _, tc := range tests {
		cmd, args, ok := ParseLeadingSlash(tc.in)
		if ok != tc.wantOK || cmd != tc.wantCmd || args != tc.wantArg {
			t.Fatalf("ParseLeadingSlash(%q) = (%q,%q,%v) want (%q,%q,%v)",
				tc.in, cmd, args, ok, tc.wantCmd, tc.wantArg, tc.wantOK)
		}
	}
}
