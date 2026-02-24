package ipc

import (
	"fmt"
	"testing"
	"time"

	"github.com/ousiassllc/moleport/internal/core"
	"github.com/ousiassllc/moleport/internal/ipc/protocol"
)

func TestToRPCError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		defaultCode int
		wantCode    int
		wantMsg     string
	}{
		{
			name:        "host not found",
			err:         fmt.Errorf("host not found"),
			defaultCode: protocol.InternalError,
			wantCode:    protocol.HostNotFound,
			wantMsg:     "host not found",
		},
		{
			name:        "rule not found",
			err:         fmt.Errorf("rule not found"),
			defaultCode: protocol.InternalError,
			wantCode:    protocol.RuleNotFound,
			wantMsg:     "rule not found",
		},
		{
			name:        "already exists",
			err:         fmt.Errorf("rule already exists"),
			defaultCode: protocol.InternalError,
			wantCode:    protocol.RuleAlreadyExists,
			wantMsg:     "rule already exists",
		},
		{
			name:        "already active",
			err:         fmt.Errorf("connection already active"),
			defaultCode: protocol.InternalError,
			wantCode:    protocol.AlreadyConnected,
			wantMsg:     "connection already active",
		},
		{
			name:        "not connected",
			err:         fmt.Errorf("host is not connected"),
			defaultCode: protocol.InternalError,
			wantCode:    protocol.NotConnected,
			wantMsg:     "host is not connected",
		},
		{
			name:        "already connected",
			err:         fmt.Errorf("host already connected"),
			defaultCode: protocol.InternalError,
			wantCode:    protocol.AlreadyConnected,
			wantMsg:     "host already connected",
		},
		{
			name:        "credential timeout",
			err:         fmt.Errorf("credential timeout"),
			defaultCode: protocol.InternalError,
			wantCode:    protocol.CredentialTimeout,
			wantMsg:     "credential timeout",
		},
		{
			name:        "credential cancelled",
			err:         fmt.Errorf("credential cancelled"),
			defaultCode: protocol.InternalError,
			wantCode:    protocol.CredentialCancelled,
			wantMsg:     "credential cancelled",
		},
		{
			name:        "generic error uses defaultCode",
			err:         fmt.Errorf("something unexpected happened"),
			defaultCode: protocol.InvalidParams,
			wantCode:    protocol.InvalidParams,
			wantMsg:     "something unexpected happened",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toRPCError(tt.err, tt.defaultCode)
			if got.Code != tt.wantCode {
				t.Errorf("Code = %d, want %d", got.Code, tt.wantCode)
			}
			if got.Message != tt.wantMsg {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMsg)
			}
		})
	}
}

func TestToHostInfo(t *testing.T) {
	tests := []struct {
		name string
		host core.SSHHost
		want protocol.HostInfo
	}{
		{
			name: "connected host",
			host: core.SSHHost{
				Name:               "prod",
				HostName:           "192.168.1.1",
				Port:               22,
				User:               "admin",
				State:              core.Connected,
				ActiveForwardCount: 3,
			},
			want: protocol.HostInfo{
				Name:               "prod",
				HostName:           "192.168.1.1",
				Port:               22,
				User:               "admin",
				State:              "connected",
				ActiveForwardCount: 3,
			},
		},
		{
			name: "disconnected host",
			host: core.SSHHost{
				Name:               "staging",
				HostName:           "10.0.0.1",
				Port:               2222,
				User:               "deploy",
				State:              core.Disconnected,
				ActiveForwardCount: 0,
			},
			want: protocol.HostInfo{
				Name:               "staging",
				HostName:           "10.0.0.1",
				Port:               2222,
				User:               "deploy",
				State:              "disconnected",
				ActiveForwardCount: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toHostInfo(tt.host)
			if got != tt.want {
				t.Errorf("toHostInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestToForwardInfo(t *testing.T) {
	tests := []struct {
		name string
		rule core.ForwardRule
		want protocol.ForwardInfo
	}{
		{
			name: "local forward rule",
			rule: core.ForwardRule{
				Name:        "web",
				Host:        "prod",
				Type:        core.Local,
				LocalPort:   8080,
				RemoteHost:  "localhost",
				RemotePort:  80,
				AutoConnect: true,
			},
			want: protocol.ForwardInfo{
				Name:        "web",
				Host:        "prod",
				Type:        "local",
				LocalPort:   8080,
				RemoteHost:  "localhost",
				RemotePort:  80,
				AutoConnect: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toForwardInfo(tt.rule)
			if got != tt.want {
				t.Errorf("toForwardInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestToSessionInfo(t *testing.T) {
	connectedAt := time.Date(2026, 2, 11, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name string
		sess core.ForwardSession
		want protocol.SessionInfo
	}{
		{
			name: "non-zero ConnectedAt formatted as RFC3339",
			sess: core.ForwardSession{
				ID: "prod-local-8080",
				Rule: core.ForwardRule{
					Name:       "web",
					Host:       "prod",
					Type:       core.Local,
					LocalPort:  8080,
					RemoteHost: "localhost",
					RemotePort: 80,
				},
				Status:         core.Active,
				ConnectedAt:    connectedAt,
				BytesSent:      1024,
				BytesReceived:  2048,
				ReconnectCount: 1,
				LastError:      "connection reset",
			},
			want: protocol.SessionInfo{
				ID:             "prod-local-8080",
				Name:           "web",
				Host:           "prod",
				Type:           "local",
				LocalPort:      8080,
				RemoteHost:     "localhost",
				RemotePort:     80,
				Status:         "active",
				ConnectedAt:    connectedAt.Format(time.RFC3339),
				BytesSent:      1024,
				BytesReceived:  2048,
				ReconnectCount: 1,
				LastError:      "connection reset",
			},
		},
		{
			name: "zero ConnectedAt results in empty string",
			sess: core.ForwardSession{
				ID: "staging-local-3000",
				Rule: core.ForwardRule{
					Name:       "api",
					Host:       "staging",
					Type:       core.Local,
					LocalPort:  3000,
					RemoteHost: "localhost",
					RemotePort: 3000,
				},
				Status:      core.Stopped,
				ConnectedAt: time.Time{},
			},
			want: protocol.SessionInfo{
				ID:         "staging-local-3000",
				Name:       "api",
				Host:       "staging",
				Type:       "local",
				LocalPort:  3000,
				RemoteHost: "localhost",
				RemotePort: 3000,
				Status:     "stopped",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toSessionInfo(tt.sess)
			if got != tt.want {
				t.Errorf("toSessionInfo() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
