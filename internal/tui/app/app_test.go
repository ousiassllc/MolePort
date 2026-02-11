package app

import (
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc"
)

func TestHostInfoToSSHHost(t *testing.T) {
	info := ipc.HostInfo{
		Name:               "prod",
		HostName:           "prod.example.com",
		Port:               22,
		User:               "deploy",
		State:              "connected",
		ActiveForwardCount: 3,
	}

	host := hostInfoToSSHHost(info)

	if host.Name != "prod" {
		t.Errorf("Name = %q, want %q", host.Name, "prod")
	}
	if host.HostName != "prod.example.com" {
		t.Errorf("HostName = %q, want %q", host.HostName, "prod.example.com")
	}
	if host.Port != 22 {
		t.Errorf("Port = %d, want %d", host.Port, 22)
	}
	if host.User != "deploy" {
		t.Errorf("User = %q, want %q", host.User, "deploy")
	}
	if host.State != core.Connected {
		t.Errorf("State = %v, want %v", host.State, core.Connected)
	}
	if host.ActiveForwardCount != 3 {
		t.Errorf("ActiveForwardCount = %d, want %d", host.ActiveForwardCount, 3)
	}
}

func TestSessionInfoToForwardSession(t *testing.T) {
	info := ipc.SessionInfo{
		ID:             "session-123",
		Name:           "web",
		Host:           "prod",
		Type:           "local",
		LocalPort:      8080,
		RemoteHost:     "localhost",
		RemotePort:     80,
		Status:         "active",
		ConnectedAt:    "2025-01-01T00:00:00Z",
		BytesSent:      1024,
		BytesReceived:  2048,
		ReconnectCount: 1,
		LastError:      "timeout",
	}

	session := sessionInfoToForwardSession(info)

	if session.ID != "session-123" {
		t.Errorf("ID = %q, want %q", session.ID, "session-123")
	}
	if session.Rule.Name != "web" {
		t.Errorf("Rule.Name = %q, want %q", session.Rule.Name, "web")
	}
	if session.Rule.Host != "prod" {
		t.Errorf("Rule.Host = %q, want %q", session.Rule.Host, "prod")
	}
	if session.Rule.Type != core.Local {
		t.Errorf("Rule.Type = %v, want %v", session.Rule.Type, core.Local)
	}
	if session.Rule.LocalPort != 8080 {
		t.Errorf("Rule.LocalPort = %d, want %d", session.Rule.LocalPort, 8080)
	}
	if session.Rule.RemoteHost != "localhost" {
		t.Errorf("Rule.RemoteHost = %q, want %q", session.Rule.RemoteHost, "localhost")
	}
	if session.Rule.RemotePort != 80 {
		t.Errorf("Rule.RemotePort = %d, want %d", session.Rule.RemotePort, 80)
	}
	if session.Status != core.Active {
		t.Errorf("Status = %v, want %v", session.Status, core.Active)
	}
	expectedTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if !session.ConnectedAt.Equal(expectedTime) {
		t.Errorf("ConnectedAt = %v, want %v", session.ConnectedAt, expectedTime)
	}
	if session.BytesSent != 1024 {
		t.Errorf("BytesSent = %d, want %d", session.BytesSent, 1024)
	}
	if session.BytesReceived != 2048 {
		t.Errorf("BytesReceived = %d, want %d", session.BytesReceived, 2048)
	}
	if session.ReconnectCount != 1 {
		t.Errorf("ReconnectCount = %d, want %d", session.ReconnectCount, 1)
	}
	if session.LastError != "timeout" {
		t.Errorf("LastError = %q, want %q", session.LastError, "timeout")
	}
}

func TestSessionInfoToForwardSession_EmptyConnectedAt(t *testing.T) {
	info := ipc.SessionInfo{
		ID:     "session-456",
		Name:   "db",
		Host:   "staging",
		Type:   "local",
		Status: "stopped",
	}

	session := sessionInfoToForwardSession(info)

	if !session.ConnectedAt.IsZero() {
		t.Errorf("ConnectedAt should be zero, got %v", session.ConnectedAt)
	}
	if session.Status != core.Stopped {
		t.Errorf("Status = %v, want %v", session.Status, core.Stopped)
	}
}

func TestSessionInfoToForwardSession_DynamicType(t *testing.T) {
	info := ipc.SessionInfo{
		ID:     "session-789",
		Name:   "socks",
		Host:   "prod",
		Type:   "dynamic",
		Status: "active",
	}

	session := sessionInfoToForwardSession(info)

	if session.Rule.Type != core.Dynamic {
		t.Errorf("Rule.Type = %v, want %v", session.Rule.Type, core.Dynamic)
	}
}

func TestParseConnectionState(t *testing.T) {
	tests := []struct {
		input string
		want  core.ConnectionState
	}{
		{"connected", core.Connected},
		{"connecting", core.Connecting},
		{"reconnecting", core.Reconnecting},
		{"error", core.ConnectionError},
		{"disconnected", core.Disconnected},
		{"unknown", core.Disconnected},
		{"", core.Disconnected},
	}

	for _, tt := range tests {
		got := parseConnectionState(tt.input)
		if got != tt.want {
			t.Errorf("parseConnectionState(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestParseSessionStatus(t *testing.T) {
	tests := []struct {
		input string
		want  core.SessionStatus
	}{
		{"active", core.Active},
		{"starting", core.Starting},
		{"reconnecting", core.SessionReconnecting},
		{"error", core.SessionError},
		{"stopped", core.Stopped},
		{"unknown", core.Stopped},
		{"", core.Stopped},
	}

	for _, tt := range tests {
		got := parseSessionStatus(tt.input)
		if got != tt.want {
			t.Errorf("parseSessionStatus(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
