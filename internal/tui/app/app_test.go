package app

import (
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
	"github.com/ousiassllc/moleport/internal/tui"
)

func TestHostInfoToSSHHost(t *testing.T) {
	host := hostInfoToSSHHost(protocol.HostInfo{
		Name: "prod", HostName: "prod.example.com", Port: 22,
		User: "deploy", State: "connected", ActiveForwardCount: 3,
	})
	if host.Name != "prod" || host.HostName != "prod.example.com" || host.Port != 22 {
		t.Errorf("basic fields: Name=%q HostName=%q Port=%d", host.Name, host.HostName, host.Port)
	}
	if host.User != "deploy" || host.State != core.Connected || host.ActiveForwardCount != 3 {
		t.Errorf("user/state: User=%q State=%v Count=%d", host.User, host.State, host.ActiveForwardCount)
	}
}

func TestSessionInfoToForwardSession(t *testing.T) {
	session := sessionInfoToForwardSession(protocol.SessionInfo{
		ID: "session-123", Name: "web", Host: "prod", Type: "local",
		LocalPort: 8080, RemoteHost: "localhost", RemotePort: 80,
		Status: "active", ConnectedAt: "2025-01-01T00:00:00Z",
		BytesSent: 1024, BytesReceived: 2048, ReconnectCount: 1, LastError: "timeout",
	})
	wantTime := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	if session.ID != "session-123" || session.Rule.Name != "web" || session.Rule.Host != "prod" {
		t.Errorf("basic fields: ID=%q Name=%q Host=%q", session.ID, session.Rule.Name, session.Rule.Host)
	}
	if session.Rule.Type != core.Local || session.Rule.LocalPort != 8080 {
		t.Errorf("type/port: Type=%v LocalPort=%d", session.Rule.Type, session.Rule.LocalPort)
	}
	if session.Rule.RemoteHost != "localhost" || session.Rule.RemotePort != 80 {
		t.Errorf("remote: Host=%q Port=%d", session.Rule.RemoteHost, session.Rule.RemotePort)
	}
	if session.Status != core.Active || !session.ConnectedAt.Equal(wantTime) {
		t.Errorf("status/time: Status=%v ConnectedAt=%v", session.Status, session.ConnectedAt)
	}
	if session.BytesSent != 1024 || session.BytesReceived != 2048 || session.ReconnectCount != 1 || session.LastError != "timeout" {
		t.Errorf("metrics: Sent=%d Recv=%d Recon=%d Err=%q",
			session.BytesSent, session.BytesReceived, session.ReconnectCount, session.LastError)
	}
}

func TestSessionInfoToForwardSession_EmptyConnectedAt(t *testing.T) {
	session := sessionInfoToForwardSession(protocol.SessionInfo{
		ID: "session-456", Name: "db", Host: "staging", Type: "local", Status: "stopped",
	})
	if !session.ConnectedAt.IsZero() {
		t.Errorf("ConnectedAt should be zero, got %v", session.ConnectedAt)
	}
	if session.Status != core.Stopped {
		t.Errorf("Status = %v, want %v", session.Status, core.Stopped)
	}
}

func TestSessionInfoToForwardSession_DynamicType(t *testing.T) {
	session := sessionInfoToForwardSession(protocol.SessionInfo{
		ID: "session-789", Name: "socks", Host: "prod", Type: "dynamic", Status: "active",
	})
	if session.Rule.Type != core.Dynamic {
		t.Errorf("Rule.Type = %v, want %v", session.Rule.Type, core.Dynamic)
	}
}

func TestParseConnectionState(t *testing.T) {
	tests := []struct {
		input string
		want  core.ConnectionState
	}{
		{"connected", core.Connected}, {"connecting", core.Connecting},
		{"reconnecting", core.Reconnecting}, {"error", core.ConnectionError},
		{"disconnected", core.Disconnected}, {"unknown", core.Disconnected}, {"", core.Disconnected},
	}
	for _, tt := range tests {
		if got := protocol.ParseConnectionState(tt.input); got != tt.want {
			t.Errorf("protocol.ParseConnectionState(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestLogOutputMsgNotDuplicated(t *testing.T) {
	m := newTestModel("test")
	result, _ := m.Update(tui.LogOutputMsg{Text: "テストメッセージ"})
	if got := result.(MainModel).dashboard.LogLineCount(); got != 1 {
		t.Errorf("LogLineCount() = %d, want 1", got)
	}
}

func TestParseSessionStatus(t *testing.T) {
	tests := []struct {
		input string
		want  core.SessionStatus
	}{
		{"active", core.Active}, {"starting", core.Starting},
		{"reconnecting", core.SessionReconnecting}, {"error", core.SessionError},
		{"stopped", core.Stopped}, {"unknown", core.Stopped}, {"", core.Stopped},
	}
	for _, tt := range tests {
		if got := protocol.ParseSessionStatus(tt.input); got != tt.want {
			t.Errorf("ParseSessionStatus(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
