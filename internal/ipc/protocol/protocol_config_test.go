package protocol

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConfigUpdateParams_PointerFields(t *testing.T) {
	path := "/custom/ssh/config"
	params := ConfigUpdateParams{
		SSHConfigPath: &path,
		Reconnect:     nil,
		Session:       nil,
		Log:           nil,
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal ConfigUpdateParams: %v", err)
	}

	// nil ポインタフィールドは omitempty で省略される
	if strings.Contains(string(data), `"reconnect"`) {
		t.Errorf("ConfigUpdateParams JSON should omit nil reconnect, got: %s", data)
	}
	if strings.Contains(string(data), `"session"`) {
		t.Errorf("ConfigUpdateParams JSON should omit nil session, got: %s", data)
	}
	if strings.Contains(string(data), `"log"`) {
		t.Errorf("ConfigUpdateParams JSON should omit nil log, got: %s", data)
	}
	if strings.Contains(string(data), `"update_check"`) {
		t.Errorf("ConfigUpdateParams JSON should omit nil update_check, got: %s", data)
	}

	var got ConfigUpdateParams
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ConfigUpdateParams: %v", err)
	}

	if got.SSHConfigPath == nil || *got.SSHConfigPath != path {
		t.Errorf("SSHConfigPath = %v, want %q", got.SSHConfigPath, path)
	}
	if got.Reconnect != nil {
		t.Errorf("Reconnect = %v, want nil", got.Reconnect)
	}
}

func TestConfigUpdateParams_AllFields(t *testing.T) {
	path := "/custom/ssh/config"
	enabled := true
	maxRetries := 5
	initialDelay := "2s"
	maxDelay := "30s"
	autoRestore := false
	level := "debug"
	file := "/tmp/test.log"

	params := ConfigUpdateParams{
		SSHConfigPath: &path,
		Reconnect: &ReconnectUpdateInfo{
			Enabled: &enabled, MaxRetries: &maxRetries,
			InitialDelay: &initialDelay, MaxDelay: &maxDelay,
		},
		Session: &SessionCfgUpdateInfo{AutoRestore: &autoRestore},
		Log:     &LogUpdateInfo{Level: &level, File: &file},
	}

	data, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("Marshal ConfigUpdateParams: %v", err)
	}

	var got ConfigUpdateParams
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ConfigUpdateParams: %v", err)
	}

	if got.Reconnect == nil || got.Reconnect.MaxRetries == nil || *got.Reconnect.MaxRetries != 5 {
		t.Errorf("Reconnect.MaxRetries = %v, want 5", got.Reconnect)
	}
	if got.Session == nil || got.Session.AutoRestore == nil || *got.Session.AutoRestore != false {
		t.Errorf("Session.AutoRestore = %v, want false", got.Session)
	}
	if got.Log == nil || got.Log.Level == nil || *got.Log.Level != "debug" {
		t.Errorf("Log.Level = %v, want debug", got.Log)
	}
}

func TestDaemonStatusResult_JSONRoundtrip(t *testing.T) {
	original := DaemonStatusResult{
		Version:              "v0.2.0",
		PID:                  12345,
		StartedAt:            "2026-02-11T10:00:00Z",
		Uptime:               "2h 30m",
		ConnectedClients:     2,
		ActiveSSHConnections: 3,
		ActiveForwards:       5,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal DaemonStatusResult: %v", err)
	}

	var got DaemonStatusResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal DaemonStatusResult: %v", err)
	}

	if got != original {
		t.Errorf("DaemonStatusResult roundtrip: got %+v, want %+v", got, original)
	}
}

func TestSSHEventNotification_JSONRoundtrip(t *testing.T) {
	original := SSHEventNotification{
		Type:  "error",
		Host:  "prod",
		Error: "connection refused",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal SSHEventNotification: %v", err)
	}

	var got SSHEventNotification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal SSHEventNotification: %v", err)
	}

	if got != original {
		t.Errorf("SSHEventNotification roundtrip: got %+v, want %+v", got, original)
	}
}

func TestSSHEventNotification_OmitsErrorWhenEmpty(t *testing.T) {
	notif := SSHEventNotification{Type: "connected", Host: "prod"}
	data, err := json.Marshal(notif)
	if err != nil {
		t.Fatalf("Marshal SSHEventNotification: %v", err)
	}
	if strings.Contains(string(data), `"error"`) {
		t.Errorf("SSHEventNotification JSON should omit error when empty, got: %s", data)
	}
}

func TestForwardEventNotification_JSONRoundtrip(t *testing.T) {
	original := ForwardEventNotification{
		Type:  "error",
		Name:  "web",
		Host:  "prod",
		Error: "port in use",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal ForwardEventNotification: %v", err)
	}

	var got ForwardEventNotification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal ForwardEventNotification: %v", err)
	}

	if got != original {
		t.Errorf("ForwardEventNotification roundtrip: got %+v, want %+v", got, original)
	}
}

func TestMetricsEventNotification_JSONRoundtrip(t *testing.T) {
	original := MetricsEventNotification{
		Sessions: []SessionMetrics{
			{
				Name:          "web",
				Status:        "active",
				BytesSent:     1024,
				BytesReceived: 2048,
				Uptime:        "1h 30m",
			},
			{
				Name:          "db",
				Status:        "active",
				BytesSent:     512,
				BytesReceived: 4096,
				Uptime:        "45m",
			},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal MetricsEventNotification: %v", err)
	}

	var got MetricsEventNotification
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal MetricsEventNotification: %v", err)
	}

	if len(got.Sessions) != 2 {
		t.Fatalf("len(Sessions) = %d, want 2", len(got.Sessions))
	}
	if got.Sessions[0] != original.Sessions[0] {
		t.Errorf("Sessions[0] = %+v, want %+v", got.Sessions[0], original.Sessions[0])
	}
	if got.Sessions[1] != original.Sessions[1] {
		t.Errorf("Sessions[1] = %+v, want %+v", got.Sessions[1], original.Sessions[1])
	}
}

func TestVersionCheckResult_JSONRoundtrip(t *testing.T) {
	original := VersionCheckResult{
		CurrentVersion:  "v0.3.0",
		LatestVersion:   "v0.4.0",
		UpdateAvailable: true,
		ReleaseURL:      "https://github.com/ousiassllc/moleport/releases/tag/v0.4.0",
		CheckedAt:       "2026-03-04T10:00:00Z",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal VersionCheckResult: %v", err)
	}

	var got VersionCheckResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal VersionCheckResult: %v", err)
	}

	if got != original {
		t.Errorf("VersionCheckResult roundtrip: got %+v, want %+v", got, original)
	}
}

func TestVersionCheckResult_OmitsEmptyFields(t *testing.T) {
	result := VersionCheckResult{
		CurrentVersion:  "v0.3.0",
		UpdateAvailable: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Marshal VersionCheckResult: %v", err)
	}

	s := string(data)
	if strings.Contains(s, `"latest_version"`) {
		t.Errorf("VersionCheckResult JSON should omit empty latest_version, got: %s", s)
	}
	if strings.Contains(s, `"release_url"`) {
		t.Errorf("VersionCheckResult JSON should omit empty release_url, got: %s", s)
	}
	if strings.Contains(s, `"checked_at"`) {
		t.Errorf("VersionCheckResult JSON should omit empty checked_at, got: %s", s)
	}
}

func TestUpdateCheckInfo_JSONRoundtrip(t *testing.T) {
	original := UpdateCheckInfo{
		Enabled:  true,
		Interval: "24h0m0s",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal UpdateCheckInfo: %v", err)
	}

	var got UpdateCheckInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal UpdateCheckInfo: %v", err)
	}

	if got != original {
		t.Errorf("UpdateCheckInfo roundtrip: got %+v, want %+v", got, original)
	}
}
