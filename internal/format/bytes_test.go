package format

import "testing"

func TestBytes(t *testing.T) {
	tests := []struct {
		input int64
		want  string
	}{
		{-1, "-1B"},
		{0, "0B"},
		{512, "512B"},
		{1023, "1023B"},
		{1024, "1.0KB"},
		{1536, "1.5KB"},
		{1024*1024 - 1, "1024.0KB"},
		{1024 * 1024, "1.0MB"},
		{1024*1024*2 + 1024*512, "2.5MB"},
		{1024*1024*1024 - 1, "1024.0MB"},
		{1024 * 1024 * 1024, "1.0GB"},
		{1024*1024*1024 + 1024*1024*512, "1.5GB"},
	}
	for _, tt := range tests {
		if got := Bytes(tt.input); got != tt.want {
			t.Errorf("Bytes(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
