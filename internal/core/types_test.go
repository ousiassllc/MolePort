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

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.SSHConfigPath != "~/.ssh/config" {
		t.Errorf("SSHConfigPath = %q, want %q", cfg.SSHConfigPath, "~/.ssh/config")
	}
	if !cfg.Reconnect.Enabled {
		t.Error("Reconnect.Enabled should be true")
	}
	if cfg.Reconnect.MaxRetries != 10 {
		t.Errorf("Reconnect.MaxRetries = %d, want 10", cfg.Reconnect.MaxRetries)
	}
	if cfg.Reconnect.InitialDelay.Duration != 1*time.Second {
		t.Errorf("Reconnect.InitialDelay = %v, want 1s", cfg.Reconnect.InitialDelay.Duration)
	}
	if cfg.Reconnect.MaxDelay.Duration != 60*time.Second {
		t.Errorf("Reconnect.MaxDelay = %v, want 60s", cfg.Reconnect.MaxDelay.Duration)
	}
	if !cfg.Session.AutoRestore {
		t.Error("Session.AutoRestore should be true")
	}
	if cfg.Log.Level != "info" {
		t.Errorf("Log.Level = %q, want %q", cfg.Log.Level, "info")
	}
	if cfg.Log.File != "~/.config/moleport/moleport.log" {
		t.Errorf("Log.File = %q, want %q", cfg.Log.File, "~/.config/moleport/moleport.log")
	}
	if cfg.Forwards != nil {
		t.Errorf("Forwards should be nil, got %v", cfg.Forwards)
	}
}

func TestConfig_YAMLRoundtrip(t *testing.T) {
	original := Config{
		SSHConfigPath: "~/.ssh/config",
		Reconnect: ReconnectConfig{
			Enabled:      true,
			MaxRetries:   5,
			InitialDelay: Duration{Duration: 2 * time.Second},
			MaxDelay:     Duration{Duration: 30 * time.Second},
		},
		Session: SessionConfig{AutoRestore: true},
		Log:     LogConfig{Level: "debug", File: "/tmp/test.log"},
		Forwards: []ForwardRule{
			{
				Name:        "test-web",
				Host:        "prod-server",
				Type:        Local,
				LocalPort:   8080,
				RemoteHost:  "localhost",
				RemotePort:  80,
				AutoConnect: true,
			},
			{
				Name:        "proxy",
				Host:        "staging",
				Type:        Dynamic,
				LocalPort:   1080,
				AutoConnect: false,
			},
		},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal Config: %v", err)
	}

	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal Config: %v", err)
	}

	if got.SSHConfigPath != original.SSHConfigPath {
		t.Errorf("SSHConfigPath = %q, want %q", got.SSHConfigPath, original.SSHConfigPath)
	}
	if got.Reconnect.MaxRetries != original.Reconnect.MaxRetries {
		t.Errorf("Reconnect.MaxRetries = %d, want %d", got.Reconnect.MaxRetries, original.Reconnect.MaxRetries)
	}
	if got.Reconnect.InitialDelay.Duration != original.Reconnect.InitialDelay.Duration {
		t.Errorf("Reconnect.InitialDelay = %v, want %v", got.Reconnect.InitialDelay.Duration, original.Reconnect.InitialDelay.Duration)
	}
	if len(got.Forwards) != 2 {
		t.Fatalf("len(Forwards) = %d, want 2", len(got.Forwards))
	}
	if got.Forwards[0].Type != Local {
		t.Errorf("Forwards[0].Type = %v, want %v", got.Forwards[0].Type, Local)
	}
	if got.Forwards[1].Type != Dynamic {
		t.Errorf("Forwards[1].Type = %v, want %v", got.Forwards[1].Type, Dynamic)
	}
	// dynamic の場合、RemoteHost/RemotePort は omitempty で省略される
	if got.Forwards[1].RemoteHost != "" {
		t.Errorf("Forwards[1].RemoteHost = %q, want empty", got.Forwards[1].RemoteHost)
	}
	if got.Forwards[1].RemotePort != 0 {
		t.Errorf("Forwards[1].RemotePort = %d, want 0", got.Forwards[1].RemotePort)
	}
}

func TestSSHEventType_String(t *testing.T) {
	tests := []struct {
		et   SSHEventType
		want string
	}{
		{SSHEventConnected, "Connected"},
		{SSHEventDisconnected, "Disconnected"},
		{SSHEventReconnecting, "Reconnecting"},
		{SSHEventPendingAuth, "PendingAuth"},
		{SSHEventError, "Error"},
		{SSHEventType(99), "SSHEventType(99)"},
	}
	for _, tt := range tests {
		if got := tt.et.String(); got != tt.want {
			t.Errorf("SSHEventType(%d).String() = %q, want %q", int(tt.et), got, tt.want)
		}
	}
}

func TestForwardEventType_String(t *testing.T) {
	tests := []struct {
		et   ForwardEventType
		want string
	}{
		{ForwardEventStarted, "Started"},
		{ForwardEventStopped, "Stopped"},
		{ForwardEventError, "Error"},
		{ForwardEventMetricsUpdated, "MetricsUpdated"},
		{ForwardEventType(99), "ForwardEventType(99)"},
	}
	for _, tt := range tests {
		if got := tt.et.String(); got != tt.want {
			t.Errorf("ForwardEventType(%d).String() = %q, want %q", int(tt.et), got, tt.want)
		}
	}
}

func TestCredentialType_Constants(t *testing.T) {
	tests := []struct {
		ct   CredentialType
		want string
	}{
		{CredentialPassword, "password"},
		{CredentialPassphrase, "passphrase"},
		{CredentialKeyboardInteractive, "keyboard-interactive"},
	}
	for _, tt := range tests {
		if got := string(tt.ct); got != tt.want {
			t.Errorf("CredentialType = %q, want %q", got, tt.want)
		}
	}
}
