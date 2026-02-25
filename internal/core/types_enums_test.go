package core

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestConnectionState_String(t *testing.T) {
	tests := []struct {
		state ConnectionState
		want  string
	}{
		{Disconnected, "Disconnected"},
		{Connecting, "Connecting"},
		{Connected, "Connected"},
		{Reconnecting, "Reconnecting"},
		{PendingAuth, "PendingAuth"},
		{ConnectionError, "Error"},
		{ConnectionState(99), "ConnectionState(99)"},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("ConnectionState(%d).String() = %q, want %q", int(tt.state), got, tt.want)
		}
	}
}

func TestSessionStatus_String(t *testing.T) {
	tests := []struct {
		status SessionStatus
		want   string
	}{
		{Stopped, "Stopped"},
		{Starting, "Starting"},
		{Active, "Active"},
		{SessionReconnecting, "Reconnecting"},
		{SessionError, "Error"},
		{SessionStatus(99), "SessionStatus(99)"},
	}
	for _, tt := range tests {
		if got := tt.status.String(); got != tt.want {
			t.Errorf("SessionStatus(%d).String() = %q, want %q", int(tt.status), got, tt.want)
		}
	}
}

func TestForwardType_String(t *testing.T) {
	tests := []struct {
		ft   ForwardType
		want string
	}{
		{Local, "local"},
		{Remote, "remote"},
		{Dynamic, "dynamic"},
		{ForwardType(99), "ForwardType(99)"},
	}
	for _, tt := range tests {
		if got := tt.ft.String(); got != tt.want {
			t.Errorf("ForwardType(%d).String() = %q, want %q", int(tt.ft), got, tt.want)
		}
	}
}

func TestParseForwardType(t *testing.T) {
	tests := []struct {
		input   string
		want    ForwardType
		wantErr bool
	}{
		{"local", Local, false},
		{"remote", Remote, false},
		{"dynamic", Dynamic, false},
		{"unknown", 0, true},
		{"", 0, true},
		{"LOCAL", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseForwardType(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseForwardType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseForwardType(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestForwardType_YAMLRoundtrip(t *testing.T) {
	types := []ForwardType{Local, Remote, Dynamic}
	for _, ft := range types {
		data, err := yaml.Marshal(ft)
		if err != nil {
			t.Fatalf("Marshal ForwardType %v: %v", ft, err)
		}

		var got ForwardType
		if err := yaml.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal ForwardType from %q: %v", string(data), err)
		}

		if got != ft {
			t.Errorf("ForwardType YAML roundtrip: got %v, want %v", got, ft)
		}
	}
}

func TestDuration_YAMLRoundtrip(t *testing.T) {
	durations := []time.Duration{
		1 * time.Second,
		60 * time.Second,
		500 * time.Millisecond,
		2*time.Hour + 30*time.Minute,
	}
	for _, d := range durations {
		original := Duration{Duration: d}
		data, err := yaml.Marshal(original)
		if err != nil {
			t.Fatalf("Marshal Duration %v: %v", d, err)
		}

		var got Duration
		if err := yaml.Unmarshal(data, &got); err != nil {
			t.Fatalf("Unmarshal Duration from %q: %v", string(data), err)
		}

		if got.Duration != original.Duration {
			t.Errorf("Duration YAML roundtrip: got %v, want %v", got.Duration, original.Duration)
		}
	}
}

func TestDuration_UnmarshalYAML_Invalid(t *testing.T) {
	var d Duration
	err := yaml.Unmarshal([]byte(`"not-a-duration"`), &d)
	if err == nil {
		t.Error("expected error for invalid duration string, got nil")
	}
}

func TestForwardType_UnmarshalYAML_Invalid(t *testing.T) {
	var ft ForwardType
	err := yaml.Unmarshal([]byte(`"invalid"`), &ft)
	if err == nil {
		t.Error("expected error for invalid forward type, got nil")
	}
}
