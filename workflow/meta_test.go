package workflow

import "testing"

func TestTranscriptSummaryMode(t *testing.T) {
	tests := []struct {
		meta map[string]any
		want bool
	}{
		{nil, false},
		{map[string]any{}, false},
		{map[string]any{"transcript_mode": "summary"}, true},
		{map[string]any{"transcript_mode": "SUMMARY"}, true},
		{map[string]any{"transcript_mode": "full"}, false},
		{map[string]any{"transcript_summary": true}, true},
		{map[string]any{"transcript_summary": "true"}, true},
		{map[string]any{"transcript_summary": false}, false},
	}
	for _, tc := range tests {
		if got := TranscriptSummaryMode(tc.meta); got != tc.want {
			t.Fatalf("meta=%v got %v want %v", tc.meta, got, tc.want)
		}
	}
}
