package format

import (
	"testing"
	"time"
)

func TestDuration(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{-time.Second, "0s"},
		{0, "0s"},
		{12 * time.Second, "12s"},
		{59*time.Second + 900*time.Millisecond, "59s"},
		{time.Minute, "1m 0s"},
		{5*time.Minute + 30*time.Second, "5m 30s"},
		{time.Hour, "1h 0m"},
		{2*time.Hour + 15*time.Minute + 59*time.Second, "2h 15m"},
		{24 * time.Hour, "1d 0h"},
		{50*time.Hour + 30*time.Minute, "2d 2h"},
	}
	for _, tt := range tests {
		if got := Duration(tt.input); got != tt.want {
			t.Errorf("Duration(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
