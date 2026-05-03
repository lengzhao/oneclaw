package observe

import (
	"log/slog"
	"testing"
)

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in    string
		want  slog.Level
		wantErr bool
	}{
		{"", slog.LevelInfo, false},
		{"info", slog.LevelInfo, false},
		{"DEBUG", slog.LevelDebug, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"bogus", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseLevel(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ParseLevel(%q): want error", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseLevel(%q): %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("ParseLevel(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestSetup_JSON(t *testing.T) {
	// Mutates slog.Default; keep serial within package tests.
	if err := Setup(LogOptions{Level: "debug", Format: "json", Output: ioDiscard}); err != nil {
		t.Fatal(err)
	}
}

func TestSetup_badFormat(t *testing.T) {
	err := Setup(LogOptions{Format: "xml", Output: ioDiscard})
	if err == nil {
		t.Fatal("want error")
	}
}

type discardWriter struct{}

func (discardWriter) Write(p []byte) (int, error) { return len(p), nil }

var ioDiscard = discardWriter{}
