package statuscmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestRunStatus_SessionGet_WithRemoteHost(t *testing.T) {
	stubConnectDaemon(t)
	stubMockResponses(t, map[string]json.RawMessage{
		"session.get": mustJSON(t, protocol.SessionInfo{
			Name:       "test-session",
			Host:       "myhost",
			Type:       "local",
			LocalPort:  8080,
			RemoteHost: "remote.example.com",
			RemotePort: 3306,
			Status:     protocol.SessionActive,
		}),
	})

	output := captureStdout(t, func() { RunStatus("", []string{"test-session"}) })

	if !strings.Contains(output, "remote.example.com") {
		t.Errorf("output should contain remote host, got %q", output)
	}
	if !strings.Contains(output, "3306") {
		t.Errorf("output should contain remote port, got %q", output)
	}
}

func TestRunStatus_SessionGet_WithConnectedAt(t *testing.T) {
	stubConnectDaemon(t)
	stubMockResponses(t, map[string]json.RawMessage{
		"session.get": mustJSON(t, protocol.SessionInfo{
			Name:        "test-session",
			Host:        "myhost",
			Type:        "local",
			LocalPort:   8080,
			Status:      protocol.SessionActive,
			ConnectedAt: "2026-01-01T00:00:00Z",
		}),
	})

	output := captureStdout(t, func() { RunStatus("", []string{"test-session"}) })

	if !strings.Contains(output, "2026-01-01T00:00:00Z") {
		t.Errorf("output should contain connected_at time, got %q", output)
	}
}

func TestRunStatus_SessionGet_WithReconnectCountAndLastError(t *testing.T) {
	stubConnectDaemon(t)
	stubMockResponses(t, map[string]json.RawMessage{
		"session.get": mustJSON(t, protocol.SessionInfo{
			Name:           "test-session",
			Host:           "myhost",
			Type:           "local",
			LocalPort:      8080,
			Status:         protocol.SessionActive,
			ReconnectCount: 3,
			LastError:      "connection reset",
			BytesSent:      1048576,
			BytesReceived:  2097152,
		}),
	})

	output := captureStdout(t, func() { RunStatus("", []string{"test-session"}) })

	if !strings.Contains(output, "3") {
		t.Errorf("output should contain reconnect count, got %q", output)
	}
	if !strings.Contains(output, "connection reset") {
		t.Errorf("output should contain last error, got %q", output)
	}
}

func TestRunStatus_SessionGet_WithAllFields(t *testing.T) {
	stubConnectDaemon(t)
	stubMockResponses(t, map[string]json.RawMessage{
		"session.get": mustJSON(t, protocol.SessionInfo{
			Name:           "full-session",
			Host:           "myhost",
			Type:           "local",
			LocalPort:      9090,
			RemoteHost:     "db.example.com",
			RemotePort:     5432,
			Status:         protocol.SessionActive,
			ConnectedAt:    "2026-03-15T10:00:00Z",
			BytesSent:      2097152,
			BytesReceived:  4194304,
			ReconnectCount: 5,
			LastError:      "timeout",
		}),
	})

	output := captureStdout(t, func() { RunStatus("", []string{"full-session"}) })

	// 全フィールドが出力されることを確認
	for _, want := range []string{"full-session", "myhost", "local", "9090", "db.example.com", "5432", "2026-03-15T10:00:00Z", "5", "timeout"} {
		if !strings.Contains(output, want) {
			t.Errorf("output should contain %q, got %q", want, output)
		}
	}
}

func TestRunStatus_SessionGet_NoOptionalFields(t *testing.T) {
	stubConnectDaemon(t)
	stubMockResponses(t, map[string]json.RawMessage{
		"session.get": mustJSON(t, protocol.SessionInfo{
			Name:      "minimal-session",
			Host:      "myhost",
			Type:      "local",
			LocalPort: 8080,
			Status:    protocol.SessionStopped,
			// RemoteHost, ConnectedAt, ReconnectCount, LastError はゼロ値
		}),
	})

	output := captureStdout(t, func() { RunStatus("", []string{"minimal-session"}) })

	if !strings.Contains(output, "minimal-session") {
		t.Errorf("output should contain session name, got %q", output)
	}
	// RemoteHost が空なので "remote" 行は含まれない（出力にはリモート情報なし）
	// ReconnectCount=0, LastError="" なので再接続やエラー行は含まれない
}

func TestRunStatus_Summary_WithPendingAuth(t *testing.T) {
	stubMockResponses(t, map[string]json.RawMessage{
		"daemon.status": mustJSON(t, protocol.DaemonStatusResult{
			PID:    12345,
			Uptime: "1h 30m",
		}),
		"host.list": mustJSON(t, protocol.HostListResult{
			Hosts: []protocol.HostInfo{
				{Name: "host1", State: protocol.StateConnected},
				{Name: "host2", State: protocol.StatePendingAuth},
				{Name: "host3", State: "disconnected"},
			},
		}),
		"session.list": mustJSON(t, protocol.SessionListResult{
			Sessions: []protocol.SessionInfo{
				{Name: "s1", Status: protocol.SessionActive, BytesSent: 1024, BytesReceived: 2048},
				{Name: "s2", Status: protocol.SessionStopped, BytesSent: 512, BytesReceived: 256},
			},
		}),
	})

	configDir := setupMockDaemonDir(t)
	output := captureStdout(t, func() { RunStatus(configDir, []string{}) })

	if !strings.Contains(output, "12345") {
		t.Errorf("output should contain daemon PID, got %q", output)
	}
	if output == "" {
		t.Error("output should not be empty")
	}
}

func TestRunStatus_Summary_NoPendingAuth(t *testing.T) {
	stubMockResponses(t, map[string]json.RawMessage{
		"daemon.status": mustJSON(t, protocol.DaemonStatusResult{
			PID:    11111,
			Uptime: "2h",
		}),
		"host.list": mustJSON(t, protocol.HostListResult{
			Hosts: []protocol.HostInfo{
				{Name: "host1", State: protocol.StateConnected},
				{Name: "host2", State: protocol.StateConnected},
			},
		}),
		"session.list": mustJSON(t, protocol.SessionListResult{
			Sessions: []protocol.SessionInfo{},
		}),
	})

	configDir := setupMockDaemonDir(t)
	output := captureStdout(t, func() { RunStatus(configDir, []string{}) })

	if output == "" {
		t.Error("output should not be empty for summary with no pending auth hosts")
	}
}

func TestRunStatus_Summary_JSON(t *testing.T) {
	stubMockResponses(t, map[string]json.RawMessage{
		"daemon.status": mustJSON(t, protocol.DaemonStatusResult{
			PID:    99999,
			Uptime: "5m",
		}),
		"host.list": mustJSON(t, protocol.HostListResult{
			Hosts: []protocol.HostInfo{
				{Name: "h1", State: protocol.StateConnected},
			},
		}),
		"session.list": mustJSON(t, protocol.SessionListResult{
			Sessions: []protocol.SessionInfo{
				{Name: "s1", Status: protocol.SessionActive, BytesSent: 100, BytesReceived: 200},
			},
		}),
	})

	configDir := setupMockDaemonDir(t)
	output := captureStdout(t, func() { RunStatus(configDir, []string{"-json"}) })

	if !strings.Contains(output, `"daemon"`) {
		t.Errorf("JSON output should contain 'daemon' key, got %q", output)
	}
	if !strings.Contains(output, `"hosts"`) {
		t.Errorf("JSON output should contain 'hosts' key, got %q", output)
	}
	if !strings.Contains(output, `"sessions"`) {
		t.Errorf("JSON output should contain 'sessions' key, got %q", output)
	}
}

func TestRunStatus_Summary_EmptyHostsAndSessions(t *testing.T) {
	stubMockResponses(t, map[string]json.RawMessage{
		"daemon.status": mustJSON(t, protocol.DaemonStatusResult{
			PID:    22222,
			Uptime: "0m",
		}),
		"host.list":    mustJSON(t, protocol.HostListResult{}),
		"session.list": mustJSON(t, protocol.SessionListResult{}),
	})

	configDir := setupMockDaemonDir(t)
	output := captureStdout(t, func() { RunStatus(configDir, []string{}) })

	if output == "" {
		t.Error("output should not be empty even with no hosts/sessions")
	}
}

func TestRunStatus_Summary_MixedSessionStatuses(t *testing.T) {
	stubMockResponses(t, map[string]json.RawMessage{
		"daemon.status": mustJSON(t, protocol.DaemonStatusResult{
			PID:    33333,
			Uptime: "10m",
		}),
		"host.list": mustJSON(t, protocol.HostListResult{
			Hosts: []protocol.HostInfo{
				{Name: "h1", State: protocol.StateConnected},
			},
		}),
		"session.list": mustJSON(t, protocol.SessionListResult{
			Sessions: []protocol.SessionInfo{
				{Name: "s1", Status: protocol.SessionActive, BytesSent: 1000, BytesReceived: 2000},
				{Name: "s2", Status: protocol.SessionActive, BytesSent: 3000, BytesReceived: 4000},
				{Name: "s3", Status: protocol.SessionStopped, BytesSent: 500, BytesReceived: 500},
			},
		}),
	})

	configDir := setupMockDaemonDir(t)
	output := captureStdout(t, func() { RunStatus(configDir, []string{}) })

	if output == "" {
		t.Error("output should not be empty")
	}
}
