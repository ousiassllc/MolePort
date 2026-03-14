package core

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

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
	if cfg.Reconnect.KeepAliveInterval.Duration != 30*time.Second {
		t.Errorf("Reconnect.KeepAliveInterval = %v, want 30s", cfg.Reconnect.KeepAliveInterval.Duration)
	}
	if cfg.Hosts != nil {
		t.Errorf("Hosts should be nil, got %v", cfg.Hosts)
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
	if !cfg.UpdateCheck.Enabled {
		t.Error("UpdateCheck.Enabled should be true")
	}
	if cfg.UpdateCheck.Interval.Duration != 24*time.Hour {
		t.Errorf("UpdateCheck.Interval = %v, want 24h0m0s", cfg.UpdateCheck.Interval.Duration)
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
		Session:     SessionConfig{AutoRestore: true},
		Log:         LogConfig{Level: "debug", File: "/tmp/test.log"},
		UpdateCheck: UpdateCheckConfig{Enabled: true, Interval: Duration{Duration: 12 * time.Hour}},
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
	if got.UpdateCheck.Enabled != original.UpdateCheck.Enabled {
		t.Errorf("UpdateCheck.Enabled = %v, want %v", got.UpdateCheck.Enabled, original.UpdateCheck.Enabled)
	}
	if got.UpdateCheck.Interval.Duration != original.UpdateCheck.Interval.Duration {
		t.Errorf("UpdateCheck.Interval = %v, want %v", got.UpdateCheck.Interval.Duration, original.UpdateCheck.Interval.Duration)
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

func TestConfig_YAMLRoundtrip_WithHosts(t *testing.T) {
	enabled := true
	maxRetries := 3
	original := Config{
		SSHConfigPath: "~/.ssh/config",
		Reconnect: ReconnectConfig{
			Enabled:           true,
			MaxRetries:        10,
			InitialDelay:      Duration{Duration: 1 * time.Second},
			MaxDelay:          Duration{Duration: 60 * time.Second},
			KeepAliveInterval: Duration{Duration: 30 * time.Second},
		},
		Hosts: map[string]HostConfig{
			"prod": {
				Reconnect: &ReconnectOverride{
					Enabled:    &enabled,
					MaxRetries: &maxRetries,
					MaxDelay:   &Duration{Duration: 120 * time.Second},
				},
			},
		},
		Session: SessionConfig{AutoRestore: true},
		Log:     LogConfig{Level: "info", File: "/tmp/test.log"},
	}

	data, err := yaml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal Config: %v", err)
	}

	var got Config
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal Config: %v", err)
	}

	if got.Reconnect.KeepAliveInterval.Duration != 30*time.Second {
		t.Errorf("Reconnect.KeepAliveInterval = %v, want 30s", got.Reconnect.KeepAliveInterval.Duration)
	}

	hc, ok := got.Hosts["prod"]
	if !ok {
		t.Fatal("Hosts[\"prod\"] not found")
	}
	if hc.Reconnect == nil {
		t.Fatal("Hosts[\"prod\"].Reconnect is nil")
	}
	if hc.Reconnect.Enabled == nil || *hc.Reconnect.Enabled != true {
		t.Errorf("Hosts[\"prod\"].Reconnect.Enabled = %v, want true", hc.Reconnect.Enabled)
	}
	if hc.Reconnect.MaxRetries == nil || *hc.Reconnect.MaxRetries != 3 {
		t.Errorf("Hosts[\"prod\"].Reconnect.MaxRetries = %v, want 3", hc.Reconnect.MaxRetries)
	}
	if hc.Reconnect.InitialDelay != nil {
		t.Errorf("Hosts[\"prod\"].Reconnect.InitialDelay should be nil, got %v", hc.Reconnect.InitialDelay)
	}
	if hc.Reconnect.MaxDelay == nil || hc.Reconnect.MaxDelay.Duration != 120*time.Second {
		t.Errorf("Hosts[\"prod\"].Reconnect.MaxDelay = %v, want 2m0s", hc.Reconnect.MaxDelay)
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		port    int
		wantErr bool
	}{
		{0, true},
		{-1, true},
		{1, false},
		{80, false},
		{65535, false},
		{65536, true},
	}
	for _, tt := range tests {
		err := ValidatePort(tt.port)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidatePort(%d) error = %v, wantErr %v", tt.port, err, tt.wantErr)
		}
	}
}
